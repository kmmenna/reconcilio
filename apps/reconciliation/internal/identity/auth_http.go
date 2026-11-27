package identity

import (
	"context"
	"errors"
	"log/slog"
	"uuid"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"
)

// RequireUser authenticates a Bearer session for an HTTP adapter.
func (s *Service) RequireUser(ctx context.Context, authorization string) (pgtype.UUID, error) {
	profile, err := s.authorizedProfile(ctx, authorization)
	if err != nil {
		return pgtype.UUID{}, err
	}
	id, err := uuid.Parse(profile.ID)
	if err != nil {
		slog.ErrorContext(ctx, "Could not read current user ID", "error", err)
		return pgtype.UUID{}, huma.Error500InternalServerError("The profile is temporarily unavailable.")
	}
	return pgtype.UUID{Bytes: id, Valid: true}, nil
}

func (s *Service) authorizedProfile(ctx context.Context, authorization string) (userProfile, error) {
	profile, err := s.authenticate(ctx, authorization)
	if errors.Is(err, errCredentials) {
		return userProfile{}, sessionRequired()
	}
	if err != nil {
		slog.ErrorContext(ctx, "Could not load current user", "error", err)
		return userProfile{}, huma.Error500InternalServerError("The profile is temporarily unavailable.")
	}
	return profile, nil
}
