package main

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/catalog"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/identity"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/reconciliation"
	"github.com/kmmenna/reconcilio/apps/reconciliation/migrations"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestManualAccountReprocessing(t *testing.T) {
	_, handler, token := jobTestAPI(t)
	var accounts []catalog.FinancialAccount
	readResponse(t, jobRequest(t, handler, http.MethodGet, "/api/financial-accounts", token, ""), http.StatusOK, &accounts)
	if len(accounts) == 0 {
		t.Fatal("expected a demo Financial Account")
	}
	path := "/api/financial-accounts/" + accounts[0].ID + "/reprocess"

	readResponse(t, jobRequest(t, handler, http.MethodPost, path, "", ""), http.StatusUnauthorized, new(any))
	missing := "/api/financial-accounts/aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa/reprocess"
	readResponse(t, jobRequest(t, handler, http.MethodPost, missing, token, ""), http.StatusNotFound, new(any))

	id := enqueueReprocessing(t, handler, token, accounts[0].ID)
	job := readJob(t, handler, token, id)
	if job.ID != id || job.Status != "queued" || job.FinancialAccountID != accounts[0].ID || job.MaxAttempts != 3 {
		t.Fatalf("unexpected queued job: %+v", job)
	}
}

func TestBusyFinancialAccountDefersReprocessing(t *testing.T) {
	pool, handler, token := jobTestAPI(t)
	firstAccount, secondAccount := twoJobAccounts(t, pool, handler, token)

	processor := reconciliation.New(pool)
	firstStarted, letFirstFinish := make(chan struct{}), make(chan struct{})
	var entered atomic.Bool
	startJobWorker(t, pool, jobProcessor(func(ctx context.Context, attempt *reconciliation.AccountAttempt) error {
		if entered.CompareAndSwap(false, true) {
			close(firstStarted)
			select {
			case <-ctx.Done():
				return context.Cause(ctx)
			case <-letFirstFinish:
			}
		}
		return processor.ProcessAccount(ctx, attempt)
	}))
	releaseFirst := sync.OnceFunc(func() { close(letFirstFinish) })
	t.Cleanup(releaseFirst)

	first := enqueueReprocessing(t, handler, token, firstAccount)
	waitForWorkerSignal(t, firstStarted)
	if got := readJob(t, handler, token, first); got.Status != "running" {
		t.Fatalf("first account is not running: %+v", got)
	}
	busy := enqueueReprocessing(t, handler, token, firstAccount)
	independent := enqueueReprocessing(t, handler, token, secondAccount)
	waitForJob(t, handler, token, busy, "deferred")
	other := waitForJob(t, handler, token, independent, "completed")
	if other.Attempt != 1 {
		t.Fatalf("independent job needed more than one attempt: %+v", other)
	}

	releaseFirst()
	waitForJob(t, handler, token, first, "completed")
	reprocessed := waitForJob(t, handler, token, busy, "completed")
	if reprocessed.ID != busy || reprocessed.Attempt != 1 {
		t.Fatalf("deferral changed the original job ID or used up an attempt: %+v", reprocessed)
	}
}

func TestExpiredAttemptCannotCommitOrReleaseSuccessorLease(t *testing.T) {
	pool, handler, token := jobTestAPI(t)
	account, _ := twoJobAccounts(t, pool, handler, token)
	importJobTestTransactions(t, pool, handler, token, account)

	processor := reconciliation.New(pool)
	arrivals := []chan struct{}{make(chan struct{}), make(chan struct{})}
	releases := []chan struct{}{make(chan struct{}), make(chan struct{})}
	var invocations atomic.Int32
	startJobWorker(t, pool, jobProcessor(func(ctx context.Context, attempt *reconciliation.AccountAttempt) error {
		i := int(invocations.Add(1)) - 1
		if i < len(arrivals) {
			close(arrivals[i])
			select {
			case <-ctx.Done():
				return context.Cause(ctx)
			case <-releases[i]:
			}
		}
		return processor.ProcessAccount(ctx, attempt)
	}))
	releaseObsolete := sync.OnceFunc(func() { close(releases[0]) })
	releaseSuccessor := sync.OnceFunc(func() { close(releases[1]) })
	t.Cleanup(releaseObsolete)
	t.Cleanup(releaseSuccessor)

	obsolete := enqueueReprocessing(t, handler, token, account)
	waitForWorkerSignal(t, arrivals[0])
	// Force expiration without advancing the production lease clock by 30 seconds.
	execJobTestSQL(t, pool, `UPDATE reconciliation_account_leases SET expires_at = clock_timestamp()
		WHERE financial_account_id = $1`, account)
	successor := enqueueReprocessing(t, handler, token, account)
	waitForWorkerSignal(t, arrivals[1])
	releaseObsolete()
	waitForJob(t, handler, token, obsolete, "retrying")

	var results []reconciliation.Summary
	readResponse(t, jobRequest(t, handler, http.MethodGet, "/api/reconciliations", token, ""), http.StatusOK, &results)
	if len(results) != 0 {
		t.Fatalf("obsolete attempt committed effects: %+v", results)
	}
	releaseSuccessor()
	succeeded := waitForJob(t, handler, token, successor, "completed")
	assertJobCounts(t, succeeded, 1, 2)
	if succeeded.Attempt != 1 {
		t.Fatalf("obsolete attempt interfered with its successor: %+v", succeeded)
	}
	retried := waitForJob(t, handler, token, obsolete, "completed")
	assertJobCounts(t, retried, 0, 0)
	if retried.Attempt != 2 {
		t.Fatalf("expected a second attempt after lease loss, got %+v", retried)
	}
}

func TestReprocessingRetriesAndKeepsPartialResults(t *testing.T) {
	pool, handler, token := jobTestAPI(t)
	retryAccount, failingAccount := twoJobAccounts(t, pool, handler, token)
	importJobTestTransactions(t, pool, handler, token, retryAccount)
	importJobTestTransactions(t, pool, handler, token, failingAccount)
	execJobTestSQL(t, pool, `CREATE SEQUENCE proposal_fault_once`)
	execJobTestSQL(t, pool, fmt.Sprintf(`
		CREATE FUNCTION fail_proposal_publication() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
		    IF NEW.financial_account_id = '%s'::uuid THEN
		        RAISE EXCEPTION 'injected terminal proposal publication failure';
		    END IF;
		    IF nextval('proposal_fault_once') = 1 THEN
		        RAISE EXCEPTION 'injected proposal publication failure';
		    END IF;
		    RETURN NEW;
		END;
		$$`, failingAccount))
	execJobTestSQL(t, pool, `CREATE TRIGGER fail_proposal_publication BEFORE INSERT ON match_proposals
		FOR EACH ROW EXECUTE FUNCTION fail_proposal_publication()`)

	retryID := enqueueReprocessing(t, handler, token, retryAccount)
	failID := enqueueReprocessing(t, handler, token, failingAccount)
	startJobWorker(t, pool, reconciliation.New(pool))

	retrying := waitForJob(t, handler, token, retryID, "retrying")
	if retrying.Attempt != 1 || retrying.CompletedAt != nil {
		t.Fatalf("unexpected active retry: %+v", retrying)
	}
	afterRetry := waitForJob(t, handler, token, retryID, "completed")
	assertJobCounts(t, afterRetry, 1, 2)
	if afterRetry.Attempt != 2 {
		t.Fatalf("expected success on the second attempt after a transient failure: %+v", afterRetry)
	}

	partial := waitForJob(t, handler, token, failID, "failed")
	assertJobCounts(t, partial, 1, 0)
	if partial.Attempt != 3 {
		t.Fatalf("expected failure after three attempts: %+v", partial)
	}
	execJobTestSQL(t, pool, `DROP TRIGGER fail_proposal_publication ON match_proposals`)
	remainingID := enqueueReprocessing(t, handler, token, failingAccount)
	assertJobCounts(t, waitForJob(t, handler, token, remainingID, "completed"), 0, 2)
	reuseID := enqueueReprocessing(t, handler, token, failingAccount)
	assertJobCounts(t, waitForJob(t, handler, token, reuseID, "completed"), 0, 0)

	if _, err := jobTestRiver(t, pool).JobDelete(t.Context(), failID); err != nil {
		t.Fatal(err)
	}
	history := readJob(t, handler, token, failID)
	assertJobCounts(t, history, 1, 0)
	if history.Status != "failed" || history.FinancialAccountID != failingAccount || !history.CompletedAt.Equal(*partial.CompletedAt) {
		t.Fatalf("failed job summary changed after reprocessing and River cleanup: %+v", history)
	}
}

type jobProcessor func(context.Context, *reconciliation.AccountAttempt) error

func (p jobProcessor) ProcessAccount(ctx context.Context, attempt *reconciliation.AccountAttempt) error {
	return p(ctx, attempt)
}

func waitForWorkerSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(10 * time.Second):
		t.Fatal("worker did not reach the expected processing stage")
	}
}

func assertJobCounts(t *testing.T, job reconciliation.JobStatus, automatic, proposals int64) {
	t.Helper()
	if job.CompletedAt == nil || job.AutomaticCount == nil || *job.AutomaticCount != automatic ||
		job.ProposalCount == nil || *job.ProposalCount != proposals {
		t.Fatalf("unexpected job summary %+v, want %d automatic Reconciliations and %d Match Proposals", job, automatic, proposals)
	}
}

func importJobTestTransactions(t *testing.T, pool *pgxpool.Pool, handler http.Handler, token, account string) {
	t.Helper()
	var sources []catalog.Source
	readResponse(t, jobRequest(t, handler, http.MethodGet, "/api/sources", token, ""), http.StatusOK, &sources)
	client := jobTestRiver(t, pool)
	for _, source := range sources {
		if source.Name != "Demo ERP" && source.Name != "Demo Card Statement" {
			continue
		}
		data, err := os.ReadFile("../../../../examples/demo/" + source.Side + ".csv")
		if err != nil {
			t.Fatal(err)
		}
		body := new(bytes.Buffer)
		form := multipart.NewWriter(body)
		for name, value := range map[string]string{
			"source_id": source.ID, "financial_account_id": account, "period_start": "2026-11-16", "period_end": "2026-11-22",
		} {
			if err := form.WriteField(name, value); err != nil {
				t.Fatal(err)
			}
		}
		file, err := form.CreateFormFile("file", "statement.csv")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(file, bytes.NewReader(data)); err != nil {
			t.Fatal(err)
		}
		if err := form.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/imports", body)
		req.Header.Set("Content-Type", form.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		var preview previewResponse
		readResponse(t, res, http.StatusCreated, &preview)
		var receipt struct {
			Jobs []struct {
				ID int64 `json:"job_id"`
			} `json:"jobs"`
		}
		readResponse(t, jobRequest(t, handler, http.MethodPost, "/api/imports/"+preview.ID+"/confirm", token, ""),
			http.StatusOK, &receipt)
		if len(receipt.Jobs) != 1 {
			t.Fatalf("expected one queued import job: %+v", receipt)
		}
		// Let the manual reprocessing job be the first to evaluate the confirmed data.
		if _, err := client.JobCancel(t.Context(), receipt.Jobs[0].ID); err != nil {
			t.Fatal(err)
		}
	}
}

func TestJobSummaryRecoversBeforeWorkerStartupAndAfterRiverCleanup(t *testing.T) {
	pool, handler, token := jobTestAPI(t)
	account, _ := twoJobAccounts(t, pool, handler, token)
	id := enqueueReprocessing(t, handler, token, account)
	client := jobTestRiver(t, pool)
	cancelled, err := client.JobCancel(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}

	startJobWorker(t, pool, reconciliation.New(pool))
	if _, err := client.JobDelete(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	got := readJob(t, handler, token, id)
	if got.Status != "cancelled" || got.FinancialAccountID != account || got.CompletedAt == nil ||
		!got.CompletedAt.Equal(*cancelled.FinalizedAt) || got.AutomaticCount == nil || *got.AutomaticCount != 0 {
		t.Fatalf("unexpected recovered summary: %+v", got)
	}
}

func TestWorkerRecoversSummaryDuringExecution(t *testing.T) {
	pool, handler, token := jobTestAPI(t)
	account, _ := twoJobAccounts(t, pool, handler, token)
	startJobWorker(t, pool, reconciliation.New(pool))
	client := jobTestRiver(t, pool)
	scheduled, err := client.Insert(t.Context(), reconciliation.ReconcileArgs{FinancialAccountID: account},
		&river.InsertOpts{ScheduledAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	id := scheduled.Job.ID
	if _, err := client.JobCancel(t.Context(), id); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	ticks := time.Tick(100 * time.Millisecond)
	for {
		var recovered bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM reconciliation_job_summaries WHERE job_id = $1
		)`, id).Scan(&recovered); err != nil {
			t.Fatal(err)
		}
		if recovered {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("worker did not recover the summary during execution")
		case <-ticks:
		}
	}
	if _, err := client.JobDelete(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	job := readJob(t, handler, token, id)
	assertJobCounts(t, job, 0, 0)
	if job.Status != "cancelled" || job.FinancialAccountID != account {
		t.Fatalf("unexpected recovered job: %+v", job)
	}
}

func jobTestRiver(t *testing.T, pool *pgxpool.Pool) *river.Client[pgx.Tx] {
	t.Helper()
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func twoJobAccounts(t *testing.T, pool *pgxpool.Pool, handler http.Handler, token string) (string, string) {
	t.Helper()
	execJobTestSQL(t, pool, `INSERT INTO financial_accounts (code, name)
		VALUES ('test-independent-account', 'Independent Account')`)
	var accounts []catalog.FinancialAccount
	readResponse(t, jobRequest(t, handler, http.MethodGet, "/api/financial-accounts", token, ""), http.StatusOK, &accounts)
	var first, second string
	for _, account := range accounts {
		switch account.Code {
		case "corporate-card":
			first = account.ID
		case "test-independent-account":
			second = account.ID
		}
	}
	if first == "" || second == "" {
		t.Fatalf("expected both Financial Accounts: %+v", accounts)
	}
	return first, second
}

func execJobTestSQL(t *testing.T, pool *pgxpool.Pool, statement string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(t.Context(), statement, args...); err != nil {
		t.Fatal(err)
	}
}

func startJobWorker(t *testing.T, pool *pgxpool.Pool, processor reconciliation.AccountProcessor) {
	t.Helper()
	worker, err := reconciliation.NewQueue(pool, processor, true)
	if err != nil {
		t.Fatal(err)
	}
	ctx, stop := context.WithCancel(t.Context())
	if err := worker.Start(ctx); err != nil {
		stop()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stop()
		<-worker.Stopped()
	})
}

func waitForJob(t *testing.T, handler http.Handler, token string, id int64, status string) reconciliation.JobStatus {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	ticks := time.Tick(50 * time.Millisecond)
	for {
		job := readJob(t, handler, token, id)
		if job.Status == status {
			return job
		}
		switch job.Status {
		case "completed", "failed", "cancelled":
			t.Fatalf("job %d ended with %q, want %q", id, job.Status, status)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for job %d to become %q, most recently %q", id, status, job.Status)
		case <-ticks:
		}
	}
}

func jobTestAPI(t *testing.T) (*pgxpool.Pool, http.Handler, string) {
	t.Helper()
	ctx := t.Context()
	database, err := postgres.Run(ctx, "postgres:18.6", postgres.BasicWaitStrategies())
	testcontainers.CleanupContainer(t, database)
	if err != nil {
		t.Fatal(err)
	}
	dsn, err := database.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}

	key := []byte(strings.Repeat("j", 32))
	identities := identity.New(pool, key)
	password, created, err := identities.BootstrapAdmin(ctx)
	if err != nil || !created {
		t.Fatalf("bootstrap administrator: created=%t error=%v", created, err)
	}
	queue, err := reconciliation.NewQueue(pool, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	handler := newHandler(pool, key, reconciliation.New(pool), queue)
	login := func(username, password string) string {
		t.Helper()
		payload, err := json.Marshal(map[string]string{"username": username, "password": password})
		if err != nil {
			t.Fatal(err)
		}
		var session struct {
			Token string `json:"access_token"`
		}
		readResponse(t, jobRequest(t, handler, http.MethodPost, "/api/auth/login", "", string(payload)), http.StatusOK, &session)
		return session.Token
	}
	adminToken := login("admin", password)
	readResponse(t, jobRequest(t, handler, http.MethodPost, "/api/users", adminToken,
		`{"username":"analyst","display_name":"Job Analyst","password":"test-job-analyst"}`), http.StatusCreated, new(any))
	return pool, handler, login("analyst", "test-job-analyst")
}

func jobRequest(t *testing.T, handler http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func enqueueReprocessing(t *testing.T, handler http.Handler, token, accountID string) int64 {
	t.Helper()
	var receipt struct {
		JobID int64 `json:"job_id"`
	}
	readResponse(t, jobRequest(t, handler, http.MethodPost, "/api/financial-accounts/"+accountID+"/reprocess", token, ""), http.StatusAccepted, &receipt)
	if receipt.JobID <= 0 {
		t.Fatalf("expected a durable job ID: %+v", receipt)
	}
	return receipt.JobID
}

func readJob(t *testing.T, handler http.Handler, token string, id int64) reconciliation.JobStatus {
	t.Helper()
	var job reconciliation.JobStatus
	readResponse(t, jobRequest(t, handler, http.MethodGet, fmt.Sprintf("/api/jobs/%d", id), token, ""), http.StatusOK, &job)
	return job
}
