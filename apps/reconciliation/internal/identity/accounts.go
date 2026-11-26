package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/identity/identitydb"
)

var (
	errUsernameTaken            = errors.New("username is already in use")
	errIncorrectCurrentPassword = errors.New("current password is incorrect")
)

func validAccountName(value string) bool {
	return strings.TrimSpace(value) != "" && !strings.ContainsFunc(value, unicode.IsControl)
}

func (s *Service) createUser(ctx context.Context, username, displayName, password string) (userProfile, error) {
	user, err := s.users.CreateUser(ctx, identitydb.CreateUserParams{
		Username:     username,
		DisplayName:  displayName,
		PasswordHash: hashPassword(password),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return userProfile{}, errUsernameTaken
	}
	if err != nil {
		return userProfile{}, fmt.Errorf("create local account: %w", err)
	}
	return userProfile(user), nil
}

func (s *Service) changeDisplayName(ctx context.Context, id, displayName string) (userProfile, error) {
	updated, err := s.users.UpdateDisplayName(ctx, identitydb.UpdateDisplayNameParams{ID: id, DisplayName: displayName})
	if errors.Is(err, pgx.ErrNoRows) {
		return userProfile{}, errCredentials
	}
	if err != nil {
		return userProfile{}, fmt.Errorf("update display name: %w", err)
	}
	return userProfile(updated), nil
}

func (s *Service) changePassword(ctx context.Context, id, currentPassword, newPassword string) error {
	currentHash, err := s.users.FindPasswordHash(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return errCredentials
	}
	if err != nil {
		return fmt.Errorf("look up current password hash: %w", err)
	}
	if !verifyPassword(currentPassword, currentHash) {
		return errIncorrectCurrentPassword
	}

	// Replace only the verified hash so a concurrent change cannot overwrite it.
	changed, err := s.users.ReplacePasswordHash(ctx, identitydb.ReplacePasswordHashParams{
		ID: id, CurrentHash: currentHash, NewHash: hashPassword(newPassword),
	})
	if err != nil {
		return fmt.Errorf("replace password hash: %w", err)
	}
	if changed == 0 {
		return errIncorrectCurrentPassword
	}
	return nil
}
