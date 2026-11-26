package identity

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/httpapi"
)

type loginInput struct {
	Body struct {
		Username string `json:"username" minLength:"1" maxLength:"100"`
		Password string `json:"password" minLength:"1" maxLength:"512"`
	}
}

type loginOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         struct {
		AccessToken string `json:"access_token"`
	}
}

type profileInput struct {
	Authorization string `header:"Authorization"`
}

type profileOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         userProfile
}

func (s *Service) RegisterHTTP(api huma.API) {
	api.OpenAPI().Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"bearer": {Type: "http", Scheme: "bearer", BearerFormat: "JWT"},
	}
	huma.Register(api, huma.Operation{
		OperationID:  "sign-in",
		Method:       http.MethodPost,
		Path:         "/api/auth/login",
		Summary:      "Sign in with a local account",
		Errors:       []int{http.StatusUnauthorized},
		MaxBodyBytes: 8192,
	}, s.signIn)
	huma.Register(api, huma.Operation{
		OperationID: "get-current-user",
		Method:      http.MethodGet,
		Path:        "/api/me",
		Summary:     "Read the signed-in user",
		Errors:      []int{http.StatusUnauthorized},
		Security:    []map[string][]string{{"bearer": {}}},
	}, s.currentUser)
}

func (s *Service) signIn(ctx context.Context, in *loginInput) (*loginOutput, error) {
	token, err := s.login(ctx, in.Body.Username, in.Body.Password)
	if errors.Is(err, errCredentials) {
		return nil, httpapi.NewProblem(http.StatusUnauthorized, "invalid_credentials", "The username or password is incorrect.")
	}
	if err != nil {
		slog.ErrorContext(ctx, "Sign-in failed", "error", err)
		return nil, huma.Error500InternalServerError("Sign-in is temporarily unavailable.")
	}

	out := &loginOutput{CacheControl: "no-store"}
	out.Body.AccessToken = token
	return out, nil
}

func (s *Service) currentUser(ctx context.Context, in *profileInput) (*profileOutput, error) {
	user, err := s.authorizedProfile(ctx, in.Authorization)
	if err != nil {
		return nil, err
	}

	return &profileOutput{CacheControl: "no-store", Body: user}, nil
}

func sessionRequired() error {
	return huma.ErrorWithHeaders(
		httpapi.NewProblem(http.StatusUnauthorized, "session_required", "Sign in to continue."),
		http.Header{"WWW-Authenticate": {"Bearer"}},
	)
}
