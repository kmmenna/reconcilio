package identity_test

import (
	"bytes"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/httpapi"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/identity"
	"github.com/kmmenna/reconcilio/apps/reconciliation/migrations"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestLocalSignIn(t *testing.T) {
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

	key := []byte(strings.Repeat("k", 32))
	accounts := identity.New(pool, key)
	password, created, err := accounts.BootstrapAdmin(ctx)
	if err != nil || !created || len(password) < 24 {
		t.Fatalf("initial bootstrap: created=%t password length=%d error=%v", created, len(password), err)
	}
	passwordAgain, created, err := accounts.BootstrapAdmin(ctx)
	if err != nil || created || passwordAgain != "" {
		t.Fatalf("repeated bootstrap: created=%t returned password=%t error=%v", created, passwordAgain != "", err)
	}
	mux := http.NewServeMux()
	accounts.RegisterHTTP(httpapi.New(mux))

	request := func(method, path, token string, body any) *httptest.ResponseRecorder {
		t.Helper()
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequestWithContext(ctx, method, path, bytes.NewReader(data))
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		return res
	}

	if res := request(http.MethodGet, "/api/me", "", nil); res.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous profile status=%d", res.Code)
	}
	invalid := request(http.MethodPost, "/api/auth/login", "", map[string]string{"username": "admin"})
	var problem struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(invalid.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if invalid.Code != http.StatusUnprocessableEntity || problem.Code != "validation_failed" {
		t.Fatalf("invalid login status=%d code=%q", invalid.Code, problem.Code)
	}
	var contract struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]any `json:"properties"`
			} `json:"schemas"`
		} `json:"components"`
	}
	spec := request(http.MethodGet, "/openapi.json", "", nil)
	if err := json.Unmarshal(spec.Body.Bytes(), &contract); err != nil {
		t.Fatal(err)
	}
	if spec.Code != http.StatusOK || contract.Components.Schemas["Problem"].Properties["code"] == nil {
		t.Fatal("the OpenAPI problem schema must describe the error code")
	}
	for _, username := range []string{"admin", "absent"} {
		res := request(http.MethodPost, "/api/auth/login", "", map[string]string{
			"username": username, "password": "not-the-password",
		})
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("invalid credentials status=%d", res.Code)
		}
	}

	res := request(http.MethodPost, "/api/auth/login", "", map[string]string{
		"username": "admin", "password": password,
	})
	if res.Code != http.StatusOK {
		t.Fatalf("login status=%d", res.Code)
	}
	var login struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &login); err != nil {
		t.Fatal(err)
	}
	res = request(http.MethodGet, "/api/me", login.AccessToken, nil)
	if res.Code != http.StatusOK {
		t.Fatalf("authenticated profile status=%d", res.Code)
	}
	var me struct {
		ID          string `json:"id"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &me); err != nil {
		t.Fatal(err)
	}
	if _, err := uuid.Parse(me.ID); err != nil || me.Username != "admin" || me.DisplayName != "Administrator" {
		t.Fatalf("unexpected profile: %+v", me)
	}

	claims := new(jwt.RegisteredClaims)
	token, err := jwt.ParseWithClaims(login.AccessToken, claims, func(_ *jwt.Token) (any, error) {
		return key, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || !token.Valid || claims.IssuedAt == nil || claims.ExpiresAt == nil {
		t.Fatalf("expected a verified time-limited JWT: %v", err)
	}
	if claims.Subject != me.ID || claims.ExpiresAt.Sub(claims.IssuedAt.Time) != 30*time.Minute {
		t.Fatal("the JWT must identify the same user and expire 30 minutes after issue")
	}
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))
	expired, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	if res := request(http.MethodGet, "/api/me", expired, nil); res.Code != http.StatusUnauthorized {
		t.Fatalf("expired token profile status=%d", res.Code)
	}
}
