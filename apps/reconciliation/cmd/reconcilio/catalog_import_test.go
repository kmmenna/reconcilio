package main

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/catalog"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/httpapi"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/identity"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/reconciliation"
	"github.com/kmmenna/reconcilio/apps/reconciliation/migrations"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestCatalogAndMixedImports(t *testing.T) {
	ctx := t.Context()
	db, err := postgres.Run(ctx, "postgres:18.6", postgres.BasicWaitStrategies())
	testcontainers.CleanupContainer(t, db)
	if err != nil {
		t.Fatal(err)
	}
	dsn, err := db.ConnectionString(ctx, "sslmode=disable")
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
	key := []byte(strings.Repeat("c", 32))
	password, _, err := identity.New(pool, key).BootstrapAdmin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	queue, err := reconciliation.NewQueue(pool, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	mux := newHandler(pool, key, reconciliation.New(pool), queue)
	var token string
	request := func(method, url, contentType string, body io.Reader) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(ctx, method, url, body)
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
	jsonRequest := func(method, url string, input any) *httptest.ResponseRecorder {
		t.Helper()
		data, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		return request(method, url, "application/json", bytes.NewReader(data))
	}

	const accountsURL, sourcesURL = "/api/financial-accounts", "/api/sources"
	var problem httpapi.Problem
	for _, url := range []string{accountsURL, sourcesURL} {
		readResponse(t, request(http.MethodGet, url, "", nil), http.StatusUnauthorized, &problem)
	}
	var session struct {
		Token string `json:"access_token"`
	}
	readResponse(t, jsonRequest(http.MethodPost, "/api/auth/login", map[string]string{
		"username": "admin", "password": password,
	}), http.StatusOK, &session)
	token = session.Token
	readResponse(t, jsonRequest(http.MethodPost, "/api/users", map[string]string{
		"username": "catalog-analyst", "display_name": "Catalog Analyst", "password": "catalog-analyst-passphrase",
	}), http.StatusCreated, new(any))
	token = ""
	readResponse(t, jsonRequest(http.MethodPost, "/api/auth/login", map[string]string{
		"username": "catalog-analyst", "password": "catalog-analyst-passphrase",
	}), http.StatusOK, &session)
	token = session.Token

	var account catalog.FinancialAccount
	readResponse(t, jsonRequest(http.MethodPost, accountsURL, map[string]string{
		"code": "operating-bank", "name": "Conta Operacional",
	}), http.StatusCreated, &account)
	accountURL := accountsURL + "/" + account.ID
	readResponse(t, jsonRequest(http.MethodPatch, accountURL, map[string]string{
		"code": "changed-code",
	}), http.StatusUnprocessableEntity, &problem)
	readResponse(t, jsonRequest(http.MethodPatch, accountURL, map[string]string{
		"name": "Conta Principal",
	}), http.StatusOK, &account)
	if account.Name != "Conta Principal" || account.Code != "operating-bank" {
		t.Fatalf("unexpected account after editing: %+v", account)
	}
	readResponse(t, jsonRequest(http.MethodPost, accountsURL, map[string]string{
		"code": account.Code, "name": "Duplicate",
	}), http.StatusConflict, &problem)
	if problem.Code != "financial_account_code_in_use" {
		t.Fatalf("unexpected duplicate code problem: %s", problem.Code)
	}
	var listedAccounts []catalog.FinancialAccount
	readResponse(t, request(http.MethodGet, accountsURL, "", nil), http.StatusOK, &listedAccounts)
	if len(listedAccounts) != 2 {
		t.Fatalf("expected the registered account and the demo account, got %+v", listedAccounts)
	}

	var source struct {
		catalog.Source
		SideEditable bool `json:"side_editable"`
	}
	readResponse(t, jsonRequest(http.MethodPost, sourcesURL, map[string]string{
		"name": "Conciliação ERP", "side": "internal",
	}), http.StatusCreated, &source)
	sourceURL := sourcesURL + "/" + source.ID
	readResponse(t, jsonRequest(http.MethodPatch, sourceURL, map[string]string{
		"name": "Sistema Financeiro", "side": "external",
	}), http.StatusOK, &source)
	if source.Name != "Sistema Financeiro" || source.Side != "external" || !source.SideEditable {
		t.Fatalf("unexpected Source after editing: %+v", source)
	}
	upload := func(accountID, start, end, data string) *httptest.ResponseRecorder {
		t.Helper()
		body := new(bytes.Buffer)
		form := multipart.NewWriter(body)
		fields := map[string]string{"source_id": source.ID}
		if start != "" {
			fields["period_start"] = start
		}
		if end != "" {
			fields["period_end"] = end
		}
		if accountID != "" {
			fields["financial_account_id"] = accountID
		}
		for key, value := range fields {
			if err := form.WriteField(key, value); err != nil {
				t.Fatal(err)
			}
		}
		file, err := form.CreateFormFile("file", "mixed.csv")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(file, data); err != nil {
			t.Fatal(err)
		}
		if err := form.Close(); err != nil {
			t.Fatal(err)
		}
		return request(http.MethodPost, "/api/imports", form.FormDataContentType(), body)
	}
	const start, end = "2026-11-01", "2026-11-05"
	mixedHeader := strings.TrimSpace(csvHeader) + ",financial_account\n"
	t.Run("reports missing and unrecognized Financial Account codes by CSV line", func(t *testing.T) {
		data := mixedHeader +
			"ERP-1,,2026-11-02,10,BRL,First,unregistered-bank\n" +
			"ERP-2,,2026-11-03,240,JPY,Second,\n" +
			"ERP-3,,2026-11-03,300,JPY,Third\n"
		var result httpapi.Problem
		readResponse(t, upload("", "", "", data), http.StatusUnprocessableEntity, &result)
		if result.Code != "invalid_csv" || len(result.Issues) != 3 ||
			result.Issues[0].Line != 2 || result.Issues[0].Field != "financial_account" ||
			result.Issues[0].Reason != "unknown_financial_account" || result.Issues[0].Value != "unregistered-bank" ||
			result.Issues[1].Line != 3 || result.Issues[1].Field != "financial_account" ||
			result.Issues[1].Reason != "missing_financial_account" ||
			result.Issues[2].Line != 4 || result.Issues[2].Field != "financial_account" ||
			result.Issues[2].Reason != "missing_financial_account" {
			t.Fatalf("unexpected row errors: %+v", result)
		}
	})

	const importsURL = "/api/imports/"
	action := func(id, action string) *httptest.ResponseRecorder {
		return request(http.MethodPost, importsURL+id+"/"+action, "", nil)
	}
	type stagedImport struct {
		ID              string                     `json:"id"`
		Status          string                     `json:"status"`
		Source          catalog.Source             `json:"source"`
		Accounts        []catalog.FinancialAccount `json:"financial_accounts"`
		AccountCoverage []accountCoverageResponse  `json:"account_coverage"`
		PeriodStart     string                     `json:"period_start"`
		PeriodEnd       string                     `json:"period_end"`
		Rows            []struct {
			ID        string `json:"financial_account_id"`
			Reference string `json:"source_reference"`
		} `json:"rows"`
	}
	var demoAccount catalog.FinancialAccount
	for _, entry := range listedAccounts {
		if entry.Code == "corporate-card" {
			demoAccount = entry
		}
	}

	t.Run("refreshes a changed Source side and confirms two accounts with independent coverage", func(t *testing.T) {
		data := mixedHeader +
			"ERP-1,,2026-11-02,10,BRL,First,corporate-card\n" +
			"ERP-1,,2026-11-03,240,JPY,Second,operating-bank\n" +
			",,2026-11-03,100,JPY,Third,operating-bank\n"
		var preview stagedImport
		first := upload("", "", "", data)
		readResponse(t, first, http.StatusCreated, &preview)
		if preview.Source.Name != "Sistema Financeiro" || preview.Source.Side != "external" ||
			len(preview.Accounts) != 2 || !slices.Contains(preview.Accounts, account) || !slices.Contains(preview.Accounts, demoAccount) ||
			len(preview.Rows) != 3 || preview.Rows[0].ID != demoAccount.ID ||
			preview.Rows[1].ID != account.ID || preview.Rows[2].ID != account.ID {
			t.Fatalf("unexpected account assignments: %+v", preview)
		}
		cardCoverage := accountCoverageResponse{AccountID: demoAccount.ID, PeriodStart: "2026-11-02", PeriodEnd: "2026-11-02", Basis: "inferred"}
		bankCoverage := accountCoverageResponse{AccountID: account.ID, PeriodStart: "2026-11-03", PeriodEnd: "2026-11-03", Basis: "inferred"}
		if preview.PeriodStart != "2026-11-02" || preview.PeriodEnd != "2026-11-03" || len(preview.AccountCoverage) != 2 ||
			!slices.Contains(preview.AccountCoverage, cardCoverage) || !slices.Contains(preview.AccountCoverage, bankCoverage) {
			t.Fatalf("unexpected per-account coverage: %+v", preview)
		}

		// Use identical bytes and a different declared period to verify confirmation replay.
		var duplicate stagedImport
		readResponse(t, upload("", start, end, data), http.StatusCreated, &duplicate)

		readResponse(t, jsonRequest(http.MethodPatch, sourceURL, map[string]string{"side": "internal"}), http.StatusOK, &source)
		var changed struct {
			Code    string `json:"code"`
			Context struct {
				Preview string `json:"preview_source_side"`
				Current string `json:"current_source_side"`
			} `json:"context"`
		}
		readResponse(t, action(preview.ID, "confirm"), http.StatusConflict, &changed)
		if changed.Code != "source_side_changed" || changed.Context.Preview != "external" || changed.Context.Current != "internal" {
			t.Fatalf("unexpected Source side conflict: %+v", changed)
		}
		var refreshed, duplicateRefreshed stagedImport
		readResponse(t, action(preview.ID, "refresh-preview"), http.StatusCreated, &refreshed)
		readResponse(t, action(duplicate.ID, "refresh-preview"), http.StatusCreated, &duplicateRefreshed)
		if refreshed.ID == preview.ID || refreshed.Source.Side != "internal" || !slices.Equal(refreshed.Rows, preview.Rows) ||
			!slices.Equal(refreshed.AccountCoverage, preview.AccountCoverage) || refreshed.PeriodStart != preview.PeriodStart || refreshed.PeriodEnd != preview.PeriodEnd {
			t.Fatalf("unexpected refreshed preview: %+v", refreshed)
		}
		if duplicateRefreshed.PeriodStart != start || duplicateRefreshed.PeriodEnd != end || len(duplicateRefreshed.AccountCoverage) != 2 ||
			!slices.Contains(duplicateRefreshed.AccountCoverage, accountCoverageResponse{AccountID: demoAccount.ID, PeriodStart: start, PeriodEnd: end, Basis: "declared"}) ||
			!slices.Contains(duplicateRefreshed.AccountCoverage, accountCoverageResponse{AccountID: account.ID, PeriodStart: start, PeriodEnd: end, Basis: "declared"}) {
			t.Fatalf("refresh changed the declared coverage: %+v", duplicateRefreshed)
		}
		if original := request(http.MethodGet, importsURL+preview.ID, "", nil); original.Code != http.StatusOK || original.Body.String() != first.Body.String() {
			t.Fatalf("refresh changed the original preview: %d %s", original.Code, original.Body)
		}

		var receipt struct {
			ID               string                    `json:"id"`
			TransactionCount int                       `json:"transaction_count"`
			AccountCoverage  []accountCoverageResponse `json:"account_coverage"`
			Jobs             []struct {
				ID      int64                    `json:"job_id"`
				Account catalog.FinancialAccount `json:"financial_account"`
			} `json:"jobs"`
		}
		confirmed := action(refreshed.ID, "confirm")
		readResponse(t, confirmed, http.StatusOK, &receipt)
		if receipt.ID != refreshed.ID || receipt.TransactionCount != 3 || len(receipt.Jobs) != 2 ||
			!slices.Equal(receipt.AccountCoverage, preview.AccountCoverage) {
			t.Fatalf("unexpected receipt: %+v", receipt)
		}
		if receipt.Jobs[0].ID == receipt.Jobs[1].ID || receipt.Jobs[0].Account.ID == receipt.Jobs[1].Account.ID {
			t.Fatalf("each Financial Account must get its own job: %+v", receipt.Jobs)
		}
		for _, job := range receipt.Jobs {
			var persisted struct {
				ID int64 `json:"id"`
			}
			readResponse(t, request(http.MethodGet, fmt.Sprintf("/api/jobs/%d", job.ID), "", nil), http.StatusOK, &persisted)
			if persisted.ID != job.ID {
				t.Fatalf("job ID did not match the persisted job: %+v", persisted)
			}
		}
		for _, replay := range []*httptest.ResponseRecorder{
			action(duplicateRefreshed.ID, "confirm"),
			upload("", "", "", data),
			upload("", "2026-11-02", "2026-11-03", data),
			request(http.MethodGet, importsURL+refreshed.ID, "", nil),
		} {
			if replay.Code != http.StatusOK || replay.Body.String() != confirmed.Body.String() {
				t.Fatalf("mixed replay changed the receipt: %d %s", replay.Code, replay.Body)
			}
		}

		var invalid httpapi.Problem
		readResponse(t, upload("", "2026-11-02", "2026-11-02", data), http.StatusUnprocessableEntity, &invalid)
		if invalid.Code != "invalid_csv" || !slices.Equal(invalid.Issues, []httpapi.Issue{
			{Line: 3, Field: "transaction_date", Reason: "outside_period"},
			{Line: 4, Field: "transaction_date", Reason: "outside_period"},
		}) {
			t.Fatalf("mixed replay must validate the submitted period: %+v", invalid)
		}

		// A reference-free row in the other account does not block this referenced overlap.
		readResponse(t, upload(demoAccount.ID, "", "", csvHeader+"ERP-2,,2026-11-02,12,BRL,Another payment\n"),
			http.StatusCreated, new(stagedImport))
		var overlap struct {
			Code    string `json:"code"`
			Context struct {
				Coverage struct {
					ImportID string                   `json:"import_id"`
					Account  catalog.FinancialAccount `json:"financial_account"`
				} `json:"coverage"`
			} `json:"context"`
		}
		readResponse(t, upload("", start, end, mixedHeader+
			"ERP-3,,2026-11-02,1,BRL,Corporate,corporate-card\n"+
			"ERP-3,,2026-11-03,2,BRL,Operating,operating-bank\n"), http.StatusConflict, &overlap)
		if overlap.Code != "import_coverage_overlap" || overlap.Context.Coverage.ImportID != refreshed.ID || overlap.Context.Coverage.Account.ID != account.ID {
			t.Fatalf("overlap did not identify the affected Financial Account: %+v", overlap)
		}
		readResponse(t, jsonRequest(http.MethodPatch, sourceURL, map[string]string{"side": "external"}), http.StatusConflict, &problem)
		if problem.Code != "source_side_locked" {
			t.Fatalf("unexpected locked Source problem: %s", problem.Code)
		}
		readResponse(t, jsonRequest(http.MethodPatch, sourceURL, map[string]string{"name": "Sistema Financeiro Atualizado"}), http.StatusOK, &source)
		if source.Name != "Sistema Financeiro Atualizado" || source.Side != "internal" || source.SideEditable {
			t.Fatalf("unexpected confirmed Source after renaming: %+v", source)
		}
	})

	t.Run("uses each account's inferred dates for preview and confirmed overlap", func(t *testing.T) {
		var prior stagedImport
		readResponse(t, upload(account.ID, "", "", csvHeader+",,2027-02-03,5,BRL,Earlier bank statement\n"), http.StatusCreated, &prior)
		readResponse(t, action(prior.ID, "confirm"), http.StatusOK, new(any))

		// The mixed range starts on the earlier bank import's date. Only the card had activity that day.
		var mixed stagedImport
		const data = "MIXED-CARD,,2027-02-03,10,BRL,Card,corporate-card\n" +
			",,2027-02-07,5,BRL,Bank,operating-bank\n"
		readResponse(t, upload("", "", "", mixedHeader+data), http.StatusCreated, &mixed)
		readResponse(t, action(mixed.ID, "confirm"), http.StatusOK, new(any))

		readResponse(t, upload(account.ID, "", "", csvHeader+",,2027-02-05,9,BRL,Bank activity before its mixed coverage\n"),
			http.StatusCreated, new(stagedImport))
		var blocked struct {
			Code    string `json:"code"`
			Context struct {
				Coverage struct {
					ImportID    string                   `json:"import_id"`
					Account     catalog.FinancialAccount `json:"financial_account"`
					PeriodStart string                   `json:"period_start"`
					PeriodEnd   string                   `json:"period_end"`
				} `json:"coverage"`
			} `json:"context"`
		}
		readResponse(t, upload("", "", "", mixedHeader+
			"MIXED-NEW-CARD,,2027-02-06,1,BRL,Card,corporate-card\n"+
			"MIXED-NEW-BANK,,2027-02-07,2,BRL,Bank,operating-bank\n"), http.StatusConflict, &blocked)
		coverage := blocked.Context.Coverage
		if blocked.Code != "import_coverage_overlap" || coverage.ImportID != mixed.ID || coverage.Account.ID != account.ID ||
			coverage.PeriodStart != "2027-02-07" || coverage.PeriodEnd != "2027-02-07" {
			t.Fatalf("expected overlap at the operating bank's inclusive endpoint: %+v", blocked)
		}
	})

	t.Run("revalidates every account before committing any account", func(t *testing.T) {
		var pending, prior stagedImport
		readResponse(t, upload("", "", "", mixedHeader+
			",,2026-12-05,8,BRL,Corporate,corporate-card\n"+
			",,2026-12-06,20,BRL,Operating,operating-bank\n"), http.StatusCreated, &pending)
		readResponse(t, upload(account.ID, "", "", csvHeader+",,2026-12-06,5,BRL,First operating import\n"),
			http.StatusCreated, &prior)
		readResponse(t, action(prior.ID, "confirm"), http.StatusOK, new(any))
		readResponse(t, action(pending.ID, "confirm"), http.StatusConflict, &problem)
		if problem.Code != "import_coverage_overlap" {
			t.Fatalf("unexpected mixed confirmation conflict: %s", problem.Code)
		}
		var stillPending stagedImport
		readResponse(t, request(http.MethodGet, importsURL+pending.ID, "", nil), http.StatusOK, &stillPending)
		if stillPending.Status != "pending" {
			t.Fatalf("mixed import did not remain pending: %+v", stillPending)
		}
		readResponse(t, upload(demoAccount.ID, "", "", csvHeader+",,2026-12-05,9,BRL,First corporate import\n"),
			http.StatusCreated, &prior)
		readResponse(t, action(prior.ID, "confirm"), http.StatusOK, new(any))
	})

	t.Run("scopes identical file replay to the exact account assignments", func(t *testing.T) {
		const start, end = "2027-01-05", "2027-01-05"
		const data = csvHeader + ",,2027-01-05,5,BRL,Statement\n"
		var first, second stagedImport
		readResponse(t, upload(account.ID, start, end, data), http.StatusCreated, &first)
		readResponse(t, action(first.ID, "confirm"), http.StatusOK, new(any))
		readResponse(t, upload(demoAccount.ID, start, end, data), http.StatusCreated, &second)
		if second.ID == first.ID || len(second.Accounts) != 1 || second.Accounts[0].ID != demoAccount.ID {
			t.Fatalf("replay selected a different account assignment: %+v", second)
		}
		readResponse(t, action(second.ID, "confirm"), http.StatusOK, new(any))
	})

	t.Run("serializes a Source side edit against confirmation", func(t *testing.T) {
		readResponse(t, jsonRequest(http.MethodPost, sourcesURL, map[string]string{
			"name": "Race Source", "side": "internal",
		}), http.StatusCreated, &source)
		var preview stagedImport
		readResponse(t, upload("", start, end, mixedHeader+
			"RACE,,2026-11-02,10,BRL,First,corporate-card\n"+
			"RACE,,2026-11-03,240,JPY,Second,operating-bank\n"),
			http.StatusCreated, &preview)
		var edit, confirm *httptest.ResponseRecorder
		var requests sync.WaitGroup
		requests.Go(func() {
			edit = jsonRequest(http.MethodPatch, sourcesURL+"/"+source.ID, map[string]string{"side": "external"})
		})
		requests.Go(func() { confirm = action(preview.ID, "confirm") })
		requests.Wait()
		if confirm.Code == http.StatusOK {
			readResponse(t, edit, http.StatusConflict, &problem)
			if problem.Code != "source_side_locked" {
				t.Fatalf("unexpected rejected Source edit: %+v", problem)
			}
		} else {
			readResponse(t, edit, http.StatusOK, &source)
			readResponse(t, confirm, http.StatusConflict, &problem)
			if source.Side != "external" || problem.Code != "source_side_changed" {
				t.Fatalf("unexpected confirmation after edit: source=%+v problem=%+v", source, problem)
			}
		}
	})

}
