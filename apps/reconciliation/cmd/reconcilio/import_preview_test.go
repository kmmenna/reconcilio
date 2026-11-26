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
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/httpapi"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/identity"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/reconciliation"
	"github.com/kmmenna/reconcilio/apps/reconciliation/migrations"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

const csvHeader = "source_reference,payment_reference,transaction_date,amount,currency,description\n"

type previewResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Source struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Side string `json:"side"`
	} `json:"source"`
	FinancialAccount struct {
		ID   string `json:"id"`
		Code string `json:"code"`
		Name string `json:"name"`
	} `json:"financial_account"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
	Rows        []struct {
		Line             int    `json:"line"`
		SourceReference  string `json:"source_reference"`
		PaymentReference string `json:"payment_reference"`
		TransactionDate  string `json:"transaction_date"`
		Amount           string `json:"amount"`
		Currency         string `json:"currency"`
		Description      string `json:"description"`
	} `json:"rows"`
}

func TestCSVImportPreview(t *testing.T) {
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

	key := []byte(strings.Repeat("p", 32))
	accounts := identity.New(pool, key)
	password, created, err := accounts.BootstrapAdmin(ctx)
	if err != nil || !created {
		t.Fatalf("bootstrap administrator: created=%t error=%v", created, err)
	}
	queue, err := reconciliation.NewQueue(pool, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	reconciler := reconciliation.New(pool)
	mux := newHandler(pool, key, reconciler, queue)
	request := func(method, path, token, contentType string, body io.Reader) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(ctx, method, path, body)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		return res
	}

	loginBody, err := json.Marshal(map[string]string{"username": "admin", "password": password})
	if err != nil {
		t.Fatal(err)
	}
	login := request(http.MethodPost, "/api/auth/login", "", "application/json", bytes.NewReader(loginBody))
	var session struct {
		Token string `json:"access_token"`
	}
	readResponse(t, login, http.StatusOK, &session)
	var financialAccounts []struct {
		ID   string `json:"id"`
		Code string `json:"code"`
		Name string `json:"name"`
	}
	var sources []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Side string `json:"side"`
	}
	readResponse(t, request(http.MethodGet, "/api/financial-accounts", session.Token, "", nil), http.StatusOK, &financialAccounts)
	readResponse(t, request(http.MethodGet, "/api/sources", session.Token, "", nil), http.StatusOK, &sources)
	if len(financialAccounts) != 1 || financialAccounts[0].Code != "corporate-card" || financialAccounts[0].Name != "Corporate Card" {
		t.Fatalf("unexpected demo financial accounts: %+v", financialAccounts)
	}
	sourcesByName := map[string]string{}
	for _, source := range sources {
		sourcesByName[source.Name] = source.Side
	}
	if len(sources) != 2 || sourcesByName["Demo ERP"] != "internal" || sourcesByName["Demo Card Statement"] != "external" {
		t.Fatalf("unexpected demo sources: %+v", sources)
	}

	accountID := financialAccounts[0].ID
	sourceID := sources[0].ID
	for _, source := range sources {
		if source.Name == "Demo ERP" {
			sourceID = source.ID
		}
	}
	upload := func(token, source, account, periodStart, periodEnd, csv string) *httptest.ResponseRecorder {
		t.Helper()
		body := new(bytes.Buffer)
		form := multipart.NewWriter(body)
		for name, value := range map[string]string{
			"source_id": source, "financial_account_id": account, "period_start": periodStart, "period_end": periodEnd,
		} {
			if err := form.WriteField(name, value); err != nil {
				t.Fatal(err)
			}
		}
		file, err := form.CreateFormFile("file", "statement.csv")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(file, csv); err != nil {
			t.Fatal(err)
		}
		if err := form.Close(); err != nil {
			t.Fatal(err)
		}
		return request(http.MethodPost, "/api/imports", token, form.FormDataContentType(), body)
	}

	const start, end = "2026-11-16", "2026-11-22"
	csv := csvHeader + "ERP-001,PAY-001,2026-11-16,-89.90,BRL,Cloud hosting\n,,2026-11-22,123,JPY,\" A note, kept as supplied \"\n"
	t.Run("uploads and reloads a pending preview", func(t *testing.T) {
		var preview previewResponse
		response := upload(session.Token, sourceID, accountID, start, end, csv)
		readResponse(t, response, http.StatusCreated, &preview)
		if _, err := uuid.Parse(preview.ID); err != nil {
			t.Fatalf("invalid import ID: %q", preview.ID)
		}
		if preview.Status != "pending" || preview.Source.ID != sourceID || preview.Source.Name != "Demo ERP" || preview.Source.Side != "internal" ||
			preview.FinancialAccount.ID != accountID || preview.FinancialAccount.Name != "Corporate Card" || preview.PeriodStart != start || preview.PeriodEnd != end {
			t.Fatalf("unexpected preview metadata: %+v", preview)
		}
		if len(preview.Rows) != 2 {
			t.Fatalf("unexpected preview rows: %+v", preview.Rows)
		}
		first, last := preview.Rows[0], preview.Rows[1]
		if first.Line != 2 || first.SourceReference != "ERP-001" || first.PaymentReference != "PAY-001" ||
			first.TransactionDate != start || first.Amount != "-89.90" || first.Currency != "BRL" || first.Description != "Cloud hosting" {
			t.Fatalf("unexpected first row: %+v", first)
		}
		if last.Line != 3 || last.SourceReference != "" || last.PaymentReference != "" ||
			last.TransactionDate != end || last.Amount != "123" || last.Currency != "JPY" || last.Description != " A note, kept as supplied " {
			t.Fatalf("unexpected last row: %+v", last)
		}
		reloaded := request(http.MethodGet, "/api/imports/"+preview.ID, session.Token, "", nil)
		if reloaded.Code != http.StatusOK || reloaded.Body.String() != response.Body.String() {
			t.Fatalf("reloaded preview differs: status=%d body=%s", reloaded.Code, reloaded.Body)
		}
	})

	t.Run("confirms a preview once and durably queues the account", func(t *testing.T) {
		var preview previewResponse
		readResponse(t, upload(session.Token, sourceID, accountID, start, end, strings.ReplaceAll(csv, "001", "TEST")), http.StatusCreated, &preview)
		path := "/api/imports/" + preview.ID
		var first, second *httptest.ResponseRecorder
		var confirmations sync.WaitGroup
		confirmations.Go(func() { first = request(http.MethodPost, path+"/confirm", session.Token, "", nil) })
		confirmations.Go(func() { second = request(http.MethodPost, path+"/confirm", session.Token, "", nil) })
		confirmations.Wait()

		var receipt struct {
			ID               string    `json:"id"`
			Status           string    `json:"status"`
			ConfirmedAt      time.Time `json:"confirmed_at"`
			TransactionCount int       `json:"transaction_count"`
			Jobs             []struct {
				ID      int64 `json:"job_id"`
				Account struct {
					ID string `json:"id"`
				} `json:"financial_account"`
			} `json:"jobs"`
		}
		readResponse(t, first, http.StatusOK, &receipt)
		if receipt.ID != preview.ID || receipt.Status != "confirmed" || receipt.ConfirmedAt.IsZero() ||
			receipt.TransactionCount != 2 || len(receipt.Jobs) != 1 || receipt.Jobs[0].ID < 1 ||
			receipt.Jobs[0].Account.ID != accountID {
			t.Fatalf("unexpected receipt: %+v", receipt)
		}
		reloaded := request(http.MethodGet, path, session.Token, "", nil)
		rt := request(http.MethodPost, path+"/confirm", session.Token, "", nil)
		for _, response := range []*httptest.ResponseRecorder{second, reloaded, rt} {
			if response.Code != http.StatusOK || response.Body.String() != first.Body.String() {
				t.Fatalf("saved receipt differs: status=%d body=%s", response.Code, response.Body)
			}
		}
		var job struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		}
		readResponse(t, request(http.MethodGet, fmt.Sprintf("/api/jobs/%d", receipt.Jobs[0].ID), session.Token, "", nil), http.StatusOK, &job)
		if job.ID != receipt.Jobs[0].ID || job.Status != "queued" {
			t.Fatalf("unexpected durable job: %+v", job)
		}
	})

	t.Run("records unique exact matches and preserves the original evidence", func(t *testing.T) {
		var externalID string
		for _, source := range sources {
			if source.Side == "external" {
				externalID = source.ID
			}
		}

		worker, err := reconciliation.NewQueue(pool, reconciler, true)
		if err != nil {
			t.Fatal(err)
		}
		workerContext, stop := context.WithCancel(t.Context())
		if err := worker.Start(workerContext); err != nil {
			stop()
			t.Fatal(err)
		}
		t.Cleanup(func() {
			stop()
			<-worker.Stopped()
		})

		stageCSV := func(source, data string) string {
			t.Helper()
			var staged previewResponse
			readResponse(t, upload(session.Token, source, accountID, start, end, data), http.StatusCreated, &staged)
			return "/api/imports/" + staged.ID
		}
		confirmImport := func(path string) int64 {
			t.Helper()
			var receipt struct {
				Jobs []struct {
					ID int64 `json:"job_id"`
				} `json:"jobs"`
			}
			readResponse(t, request(http.MethodPost, path+"/confirm", session.Token, "", nil), http.StatusOK, &receipt)
			if len(receipt.Jobs) != 1 {
				t.Fatalf("unexpected scheduled jobs: %+v", receipt.Jobs)
			}
			return receipt.Jobs[0].ID
		}
		waitForJob := func(jobID int64, wantCount int) {
			t.Helper()
			path := fmt.Sprintf("/api/jobs/%d", jobID)
			deadline, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			ticks := time.Tick(50 * time.Millisecond)
			for {
				var job struct {
					Status         string    `json:"status"`
					CompletedAt    time.Time `json:"completed_at"`
					AutomaticCount *int      `json:"automatic_count"`
				}
				readResponse(t, request(http.MethodGet, path, session.Token, "", nil), http.StatusOK, &job)
				switch job.Status {
				case "completed":
					if job.CompletedAt.IsZero() || job.AutomaticCount == nil || *job.AutomaticCount != wantCount {
						t.Fatalf("unexpected job result: %s completed at %v with count %v", path, job.CompletedAt, job.AutomaticCount)
					}
					return
				case "failed", "cancelled":
					t.Fatalf("job %s ended as %s", path, job.Status)
				}
				select {
				case <-deadline.Done():
					t.Fatalf("job %s did not complete: %s", path, job.Status)
				case <-ticks:
				}
			}
		}

		internal, err := os.ReadFile("../../../../examples/demo/internal.csv")
		if err != nil {
			t.Fatal(err)
		}
		external, err := os.ReadFile("../../../../examples/demo/external.csv")
		if err != nil {
			t.Fatal(err)
		}
		internalPath := stageCSV(sourceID, string(internal)+
			"ERP-SIGN,PAY-SIGN,2026-11-17,-7.500,KWD,Opposite sign\n"+
			"ERP-CURRENCY,PAY-CURRENCY,2026-11-17,1.00,BRL,Different currency\n"+
			"ERP-EXACT,PAY-EXACT,2026-11-17,-9223372036854775.808,IQD,Exact minor units\n")
		waitForJob(confirmImport(internalPath), 0)

		externalPath := stageCSV(externalID, string(external)+
			"CARD-SIGN,PAY-SIGN,2026-11-17,7.500,KWD,Opposite sign\n"+
			"CARD-CURRENCY,PAY-CURRENCY,2026-11-17,1.00,USD,Different currency\n"+
			"CARD-EXACT,PAY-EXACT,2026-11-17,-9223372036854775.808,IQD,Exact minor units\n")
		// The failed enqueue must leave the preview pending and both eventual reference matches unique.
		if _, err := pool.Exec(t.Context(), `
			CREATE FUNCTION reject_test_job_insert() RETURNS trigger LANGUAGE plpgsql AS $$
			BEGIN
			    RAISE EXCEPTION 'reconciliation queue unavailable in test';
			END;
			$$;
			CREATE TRIGGER reject_test_job_insert BEFORE INSERT ON river_job
			FOR EACH ROW EXECUTE FUNCTION reject_test_job_insert();
		`); err != nil {
			t.Fatal(err)
		}
		failed := request(http.MethodPost, externalPath+"/confirm", session.Token, "", nil)
		if _, err := pool.Exec(t.Context(), "DROP TRIGGER reject_test_job_insert ON river_job"); err != nil {
			t.Fatal(err)
		}
		var problem httpapi.Problem
		readResponse(t, failed, http.StatusInternalServerError, &problem)
		var restored previewResponse
		readResponse(t, request(http.MethodGet, externalPath, session.Token, "", nil), http.StatusOK, &restored)
		if restored.Status != "pending" || len(restored.Rows) != 7 {
			t.Fatalf("failed confirmation changed its staged preview: %+v", restored)
		}
		waitForJob(confirmImport(externalPath), 2)

		type summary struct {
			ID          string    `json:"id"`
			Amount      string    `json:"amount"`
			Currency    string    `json:"currency"`
			Method      string    `json:"method"`
			ConfirmedAt time.Time `json:"confirmed_at"`
		}
		var results []summary
		path := "/api/reconciliations?financial_account_id=" + accountID
		readResponse(t, request(http.MethodGet, path, session.Token, "", nil), http.StatusOK, &results)
		if len(results) != 2 {
			t.Fatalf("only the two eligible reference matches should be confirmed: %+v", results)
		}
		var matchedID string
		for _, result := range results {
			if result.Method != "automatic" || result.ConfirmedAt.IsZero() {
				t.Fatalf("unexpected decision summary: %+v", result)
			}
			switch result.Currency {
			case "BRL":
				matchedID = result.ID
				if result.Amount != "-89.90" {
					t.Fatalf("unexpected demo Money: %+v", result)
				}
			case "IQD":
				if result.Amount != "-9223372036854775.808" {
					t.Fatalf("exact minor units were not preserved: %+v", result)
				}
			default:
				t.Fatalf("unexpected currency: %+v", result)
			}
		}
		if matchedID == "" {
			t.Fatal("the demo pair was not confirmed")
		}

		detailPath := "/api/reconciliations/" + matchedID
		var detail struct {
			Actor struct {
				Kind string `json:"kind"`
			} `json:"actor"`
			Members []struct {
				Side            string `json:"side"`
				SourceReference string `json:"source_reference"`
				Description     string `json:"description"`
			} `json:"members"`
			Evidence struct {
				Reference     string `json:"payment_reference"`
				InternalCount int    `json:"internal_reference_count"`
				ExternalCount int    `json:"external_reference_count"`
			} `json:"evidence"`
		}
		initialDetail := request(http.MethodGet, detailPath, session.Token, "", nil)
		readResponse(t, initialDetail, http.StatusOK, &detail)
		if detail.Actor.Kind != "engine" || detail.Evidence.Reference != "DEMO-PAY-001" ||
			detail.Evidence.InternalCount != 1 || detail.Evidence.ExternalCount != 1 || len(detail.Members) != 2 ||
			detail.Members[0].Side != "internal" || detail.Members[0].SourceReference != "ERP-001" ||
			detail.Members[0].Description != "Cloud hosting subscription" ||
			detail.Members[1].Side != "external" || detail.Members[1].SourceReference != "CARD-001" ||
			detail.Members[1].Description != "CLOUDHOST* MONTHLY" {
			t.Fatalf("unexpected reconciliation detail: %+v", detail)
		}

		waitForJob(confirmImport(stageCSV(sourceID, csvHeader+"ERP-LATE,DEMO-PAY-001,2026-11-16,-89.90,BRL,Another internal record\n")), 0)
		waitForJob(confirmImport(stageCSV(externalID, csvHeader+"CARD-LATE,DEMO-PAY-001,2026-11-16,-89.90,BRL,Another external record\n")), 0)
		if after := request(http.MethodGet, detailPath, session.Token, "", nil); after.Body.String() != initialDetail.Body.String() {
			t.Fatalf("later duplicate changed recorded evidence: %s", after.Body)
		}
		readResponse(t, request(http.MethodGet, path, session.Token, "", nil), http.StatusOK, &results)
		if len(results) != 2 {
			t.Fatalf("late duplicates should not produce a second automatic decision: %+v", results)
		}
		readResponse(t, request(http.MethodGet, "/api/reconciliations?financial_account_id="+uuid.New().String(), session.Token, "", nil),
			http.StatusOK, &results)
		if len(results) != 0 {
			t.Fatalf("filter returned results from another Financial Account: %+v", results)
		}

		verifyProposalReview(t, accountID, func(method, path string) *httptest.ResponseRecorder {
			return request(method, path, session.Token, "", nil)
		}, func(sourceSide, csv string) {
			source := sourceID
			if sourceSide == "external" {
				source = externalID
			}
			waitForJob(confirmImport(stageCSV(source, csv)), 0)
		})
	})

	t.Run("returns line and field diagnostics for invalid rows", func(t *testing.T) {
		invalid := csvHeader +
			"ERP-001,,2026-11-16,0.1,JPY,No fractional yen\n" +
			"ERP-002,,2026-11-23,25.00,BRL,Outside coverage\n" +
			"ERP-003,,2026-11-31,3.00,ZZZ,Unknown currency and invalid date\n" +
			"ERP-004,,2026-11-18,92233720368547758.08,BRL,Too large\n" +
			"ERP-005,,2026-11-19,1e2,USD,Invalid decimal notation\n"

		response := upload(session.Token, sourceID, accountID, start, end, invalid)
		var problem httpapi.Problem
		readResponse(t, response, http.StatusUnprocessableEntity, &problem)
		if !strings.HasPrefix(response.Header().Get("Content-Type"), "application/problem+json") || problem.Code != "invalid_csv" {
			t.Fatalf("unexpected CSV problem: %+v", problem)
		}
		want := []httpapi.Issue{
			{Line: 2, Field: "amount", Reason: "invalid_precision"},
			{Line: 3, Field: "transaction_date", Reason: "outside_period"},
			{Line: 4, Field: "transaction_date", Reason: "invalid_date"},
			{Line: 4, Field: "currency", Reason: "invalid_currency"},
			{Line: 5, Field: "amount", Reason: "amount_out_of_range"},
			{Line: 6, Field: "amount", Reason: "invalid_amount"},
		}
		if !slices.Equal(problem.Issues, want) {
			t.Fatalf("issues=%+v, want %+v", problem.Issues, want)
		}
	})

	t.Run("preserves exact money and physical CSV positions", func(t *testing.T) {
		exact := "\xef\xbb\xbf" + csvHeader +
			"ERP-001,,2026-11-16,-92233720368547758.08,BRL,\"Line one\nline two\"\n" +
			"ERP-002,,2026-11-17,+7.5,KWD,Three minor digits\n" +
			"ERP-003,,2026-11-18,1.234,IQD,ISO minor digits\n" +
			"ERP-004,,2026-11-19,5.05,XCG,Caribbean guilder\n"
		var preview previewResponse
		readResponse(t, upload(session.Token, sourceID, accountID, start, end, exact), http.StatusCreated, &preview)
		if len(preview.Rows) != 4 || preview.Rows[0].Amount != "-92233720368547758.08" ||
			preview.Rows[0].Description != "Line one\nline two" || preview.Rows[1].Line != 4 ||
			preview.Rows[1].Amount != "7.500" || preview.Rows[1].Currency != "KWD" ||
			preview.Rows[2].Amount != "1.234" || preview.Rows[2].Currency != "IQD" ||
			preview.Rows[3].Amount != "5.05" || preview.Rows[3].Currency != "XCG" {
			t.Fatalf("unexpected exact money rows: %+v", preview.Rows)
		}
	})

	t.Run("rejects invalid files and selections", func(t *testing.T) {
		for _, test := range []struct {
			name, source, account, first, last, data, code string
			issue                                          httpapi.Issue
		}{
			{"wrong header", sourceID, accountID, start, end, "date,amount\n", "invalid_csv",
				httpapi.Issue{Line: 1, Field: "header", Reason: "invalid_header"}},
			{"wrong record width", sourceID, accountID, start, end, csvHeader + "a,b,c\n", "invalid_csv",
				httpapi.Issue{Line: 2, Field: "row", Reason: "invalid_column_count"}},
			{"malformed CSV", sourceID, accountID, start, end, csvHeader + "ERP-001,,2026-11-16,1,BRL,\"unterminated\n", "invalid_csv",
				httpapi.Issue{Line: 2, Field: "row", Reason: "invalid_record"}},
			{"header only", sourceID, accountID, start, end, csvHeader, "invalid_csv",
				httpapi.Issue{Field: "file", Reason: "no_rows"}},
			{"unknown source", uuid.New().String(), accountID, start, end, csv, "invalid_import_selection",
				httpapi.Issue{Field: "source_id", Reason: "unknown_source"}},
			{"unknown financial account", sourceID, uuid.New().String(), start, end, csv, "invalid_import_selection",
				httpapi.Issue{Field: "financial_account_id", Reason: "unknown_financial_account"}},
			{"invalid account ID", sourceID, "bad-id", start, end, csv, "invalid_import_selection",
				httpapi.Issue{Field: "financial_account_id", Reason: "invalid_financial_account"}},
			{"reversed period", sourceID, accountID, end, start, csv, "invalid_import_selection",
				httpapi.Issue{Field: "period_end", Reason: "invalid_period"}},
		} {
			t.Run(test.name, func(t *testing.T) {
				var problem httpapi.Problem
				readResponse(t, upload(session.Token, test.source, test.account, test.first, test.last, test.data),
					http.StatusUnprocessableEntity, &problem)
				if problem.Code != test.code || !slices.Equal(problem.Issues, []httpapi.Issue{test.issue}) {
					t.Fatalf("unexpected problem: %+v", problem)
				}
			})
		}
	})

	t.Run("returns a stable not-found problem for an unknown preview", func(t *testing.T) {
		var problem httpapi.Problem
		readResponse(t, request(http.MethodGet, "/api/imports/"+uuid.New().String(), session.Token, "", nil), http.StatusNotFound, &problem)
		if problem.Code != "preview_not_found" {
			t.Fatalf("unexpected problem: %+v", problem)
		}
	})

	t.Run("requires an authenticated analyst", func(t *testing.T) {
		var problem httpapi.Problem
		readResponse(t, request(http.MethodGet, "/api/sources", "", "", nil), http.StatusUnauthorized, &problem)
		readResponse(t, upload("", sourceID, accountID, start, end, csv), http.StatusUnauthorized, &problem)
		readResponse(t, request(http.MethodGet, "/api/imports/"+uuid.New().String(), "", "", nil), http.StatusUnauthorized, &problem)
		readResponse(t, request(http.MethodPost, "/api/imports/"+uuid.New().String()+"/confirm", "", "", nil), http.StatusUnauthorized, &problem)
		readResponse(t, request(http.MethodGet, "/api/jobs/1", "", "", nil), http.StatusUnauthorized, &problem)
		readResponse(t, request(http.MethodGet, "/api/reconciliations", "", "", nil), http.StatusUnauthorized, &problem)
		readResponse(t, request(http.MethodGet, "/api/reconciliations/"+uuid.New().String(), "", "", nil), http.StatusUnauthorized, &problem)
		if problem.Code != "session_required" {
			t.Fatalf("unexpected authentication problem: %+v", problem)
		}
	})
}

func readResponse(t *testing.T, response *httptest.ResponseRecorder, status int, body any) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status=%d, want %d, body=%s", response.Code, status, response.Body)
	}
	if err := json.Unmarshal(response.Body.Bytes(), body); err != nil {
		t.Fatal(err)
	}
}
