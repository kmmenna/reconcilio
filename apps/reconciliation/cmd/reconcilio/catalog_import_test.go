package main

import (
	"bytes"
	"encoding/json/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
}
