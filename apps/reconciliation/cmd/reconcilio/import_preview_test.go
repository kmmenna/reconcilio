package main

import (
	"bytes"
	"encoding/json/v2"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/httpapi"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/identity"
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
	mux := newHandler(pool, key)
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
			"ERP-002,,2026-11-17,+7.5,KWD,Three minor digits\n"
		var preview previewResponse
		readResponse(t, upload(session.Token, sourceID, accountID, start, end, exact), http.StatusCreated, &preview)
		if len(preview.Rows) != 2 || preview.Rows[0].Amount != "-92233720368547758.08" ||
			preview.Rows[0].Description != "Line one\nline two" || preview.Rows[1].Line != 4 ||
			preview.Rows[1].Amount != "7.500" || preview.Rows[1].Currency != "KWD" {
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
