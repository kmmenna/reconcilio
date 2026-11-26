package identity

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/httpapi"
)

type createAccountInput struct {
	Authorization string `header:"Authorization"`
	Body          struct {
		Username    string `json:"username" minLength:"1" maxLength:"100"`
		DisplayName string `json:"display_name" minLength:"1" maxLength:"120"`
		Password    string `json:"password" minLength:"8" maxLength:"512" format:"password"`
	}
}

type updateProfileInput struct {
	Authorization string `header:"Authorization"`
	Body          struct {
		DisplayName string `json:"display_name" minLength:"1" maxLength:"120"`
	}
}

type changePasswordInput struct {
	Authorization string `header:"Authorization"`
	Body          struct {
		CurrentPassword string `json:"current_password" minLength:"1" maxLength:"512" format:"password"`
		NewPassword     string `json:"new_password" minLength:"8" maxLength:"512" format:"password"`
	}
}

type changePasswordOutput struct {
	CacheControl string `header:"Cache-Control"`
}

func (s *Service) registerAccountHTTP(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "create-local-user",
		Method:        http.MethodPost,
		Path:          "/api/users",
		Summary:       "Create a local user as an administrator",
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusConflict, http.StatusUnprocessableEntity},
		Security:      []map[string][]string{{"bearer": {}}},
		MaxBodyBytes:  8192,
	}, s.createAccount)
	huma.Register(api, huma.Operation{
		OperationID:  "update-current-user",
		Method:       http.MethodPatch,
		Path:         "/api/me",
		Summary:      "Update the signed-in user's display name",
		Errors:       []int{http.StatusUnauthorized, http.StatusUnprocessableEntity},
		Security:     []map[string][]string{{"bearer": {}}},
		MaxBodyBytes: 4096,
	}, s.updateProfile)
	huma.Register(api, huma.Operation{
		OperationID:   "change-current-user-password",
		Method:        http.MethodPut,
		Path:          "/api/me/password",
		Summary:       "Change the signed-in user's password",
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnauthorized, http.StatusUnprocessableEntity},
		Security:      []map[string][]string{{"bearer": {}}},
		MaxBodyBytes:  8192,
	}, s.updatePassword)
}

func (s *Service) createAccount(ctx context.Context, in *createAccountInput) (*profileOutput, error) {
	actor, err := s.authorizedProfile(ctx, in.Authorization)
	if err != nil {
		return nil, err
	}
	if !actor.IsAdmin {
		return nil, httpapi.NewProblem(http.StatusForbidden, "administrator_required", "Ask an administrator to create the account.")
	}
	if !validAccountName(in.Body.Username) {
		return nil, httpapi.NewProblem(http.StatusUnprocessableEntity, "invalid_username", "Enter a username with visible characters and no control characters.")
	}
	if !validAccountName(in.Body.DisplayName) {
		return nil, invalidDisplayName()
	}

	created, err := s.createUser(ctx, in.Body.Username, in.Body.DisplayName, in.Body.Password)
	if errors.Is(err, errUsernameTaken) {
		return nil, httpapi.NewProblem(http.StatusConflict, "username_taken", "Choose a different username.")
	}
	if err != nil {
		slog.ErrorContext(ctx, "Could not create local account", "error", err)
		return nil, huma.Error500InternalServerError("Account creation is temporarily unavailable.")
	}

	return &profileOutput{CacheControl: "no-store", Body: created}, nil
}

func (s *Service) updateProfile(ctx context.Context, in *updateProfileInput) (*profileOutput, error) {
	actor, err := s.authorizedProfile(ctx, in.Authorization)
	if err != nil {
		return nil, err
	}
	if !validAccountName(in.Body.DisplayName) {
		return nil, invalidDisplayName()
	}

	updated, err := s.changeDisplayName(ctx, actor.ID, in.Body.DisplayName)
	if errors.Is(err, errCredentials) {
		return nil, sessionRequired()
	}
	if err != nil {
		slog.ErrorContext(ctx, "Could not update display name", "error", err)
		return nil, huma.Error500InternalServerError("The display name could not be saved.")
	}

	return &profileOutput{CacheControl: "no-store", Body: updated}, nil
}

func (s *Service) updatePassword(ctx context.Context, in *changePasswordInput) (*changePasswordOutput, error) {
	actor, err := s.authorizedProfile(ctx, in.Authorization)
	if err != nil {
		return nil, err
	}

	err = s.changePassword(ctx, actor.ID, in.Body.CurrentPassword, in.Body.NewPassword)
	if errors.Is(err, errCredentials) {
		return nil, sessionRequired()
	}
	if errors.Is(err, errIncorrectCurrentPassword) {
		return nil, httpapi.NewProblem(http.StatusUnprocessableEntity, "incorrect_current_password", "Enter your current password and try again.")
	}
	if err != nil {
		slog.ErrorContext(ctx, "Could not change password", "error", err)
		return nil, huma.Error500InternalServerError("The password could not be changed.")
	}

	return &changePasswordOutput{CacheControl: "no-store"}, nil
}

func invalidDisplayName() error {
	return httpapi.NewProblem(http.StatusUnprocessableEntity, "invalid_display_name", "Enter a display name with visible characters and no control characters.")
}
