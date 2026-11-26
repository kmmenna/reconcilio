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

	t.Run("local account management", func(t *testing.T) {
		res := request(http.MethodPost, "/api/users", login.AccessToken, map[string]string{
			"username": "Analista.Maria", "display_name": "Maria de Sá", "password": "initial-password",
		})
		if res.Code != http.StatusCreated {
			t.Fatalf("create account: status=%d body=%s", res.Code, res.Body)
		}
		user := readBody[apiProfile](t, res)
		if _, err := uuid.Parse(user.ID); err != nil || user.Username != "Analista.Maria" || user.DisplayName != "Maria de Sá" || user.IsAdmin {
			t.Fatalf("unexpected created account: %+v", user)
		}
		requireProblem(t, request(http.MethodPost, "/api/users", login.AccessToken, map[string]string{
			"username": user.Username, "display_name": "Another user", "password": "another-password",
		}), http.StatusConflict, "username_taken")
		requireProblem(t, request(http.MethodPost, "/api/users", "", map[string]string{
			"username": "unexpected-user", "display_name": "Unexpected user", "password": "another-password",
		}), http.StatusUnauthorized, "session_required")
		requireProblem(t, request(http.MethodPost, "/api/users", login.AccessToken, map[string]string{
			"username": "  ", "display_name": "Another user", "password": "another-password",
		}), http.StatusUnprocessableEntity, "invalid_username")

		signIn := request(http.MethodPost, "/api/auth/login", "", map[string]string{
			"username": user.Username, "password": "initial-password",
		})
		if signIn.Code != http.StatusOK {
			t.Fatalf("sign in as newly created analyst: status=%d body=%s", signIn.Code, signIn.Body)
		}
		token := readBody[struct {
			AccessToken string `json:"access_token"`
		}](t, signIn).AccessToken
		requireProblem(t, request(http.MethodPost, "/api/users", token, map[string]string{
			"username": "unexpected-user", "display_name": "Unexpected user", "password": "another-password",
		}), http.StatusForbidden, "administrator_required")

		t.Run("analyst edits their own display name", func(t *testing.T) {
			update := map[string]string{"display_name": "Maria  Sá Costa"}
			requireProblem(t, request(http.MethodPatch, "/api/me", "", update),
				http.StatusUnauthorized, "session_required")
			requireProblem(t, request(http.MethodPatch, "/api/me", token, map[string]string{"display_name": " "}),
				http.StatusUnprocessableEntity, "invalid_display_name")
			changed := request(http.MethodPatch, "/api/me", token, update)
			if changed.Code != http.StatusOK {
				t.Fatalf("update profile: status=%d body=%s", changed.Code, changed.Body)
			}
			want := user
			want.DisplayName = update["display_name"]
			if got := readBody[apiProfile](t, changed); got != want {
				t.Fatalf("update profile: got %+v, want %+v", got, want)
			}
			read := request(http.MethodGet, "/api/me", token, nil)
			if got := readBody[apiProfile](t, read); read.Code != http.StatusOK || got != want {
				t.Fatalf("read updated profile: status=%d got %+v, want %+v", read.Code, got, want)
			}
		})
		t.Run("analyst signs in with their new password", func(t *testing.T) {
			want := readBody[apiProfile](t, request(http.MethodGet, "/api/me", token, nil))
			passwords := map[string]string{
				"current_password": "initial-password", "new_password": "updated-password",
			}
			requireProblem(t, request(http.MethodPut, "/api/me/password", "", passwords),
				http.StatusUnauthorized, "session_required")
			requireProblem(t, request(http.MethodPut, "/api/me/password", token, map[string]string{
				"current_password": "incorrect-password", "new_password": passwords["new_password"],
			}), http.StatusUnprocessableEntity, "incorrect_current_password")
			requireProblem(t, request(http.MethodPut, "/api/me/password", token, map[string]string{
				"current_password": passwords["current_password"], "new_password": "short",
			}), http.StatusUnprocessableEntity, "validation_failed")
			changed := request(http.MethodPut, "/api/me/password", token, passwords)
			if changed.Code != http.StatusNoContent {
				t.Fatalf("change password: status=%d body=%s", changed.Code, changed.Body)
			}
			requireProblem(t, request(http.MethodPost, "/api/auth/login", "", map[string]string{
				"username": user.Username, "password": passwords["current_password"],
			}), http.StatusUnauthorized, "invalid_credentials")
			newSignIn := request(http.MethodPost, "/api/auth/login", "", map[string]string{
				"username": user.Username, "password": passwords["new_password"],
			})
			if newSignIn.Code != http.StatusOK {
				t.Fatalf("sign in with changed password: status=%d body=%s", newSignIn.Code, newSignIn.Body)
			}
			newToken := readBody[struct {
				AccessToken string `json:"access_token"`
			}](t, newSignIn).AccessToken
			newProfile := request(http.MethodGet, "/api/me", newToken, nil)
			if got := readBody[apiProfile](t, newProfile); newProfile.Code != http.StatusOK || got != want {
				t.Fatalf("read profile after password change: status=%d got %+v, want %+v", newProfile.Code, got, want)
			}
		})
	})

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

type apiProfile struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	IsAdmin     bool   `json:"is_admin"`
}

func readBody[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()
	var body T
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response (status %d): %v", response.Code, err)
	}
	return body
}

func requireProblem(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	problem := readBody[struct {
		Code string `json:"code"`
	}](t, response)
	if response.Code != status || problem.Code != code {
		t.Fatalf("problem: status=%d code=%q, want status=%d code=%q", response.Code, problem.Code, status, code)
	}
}
