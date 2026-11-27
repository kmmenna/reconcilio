package identity

import (
	"context"
	"strings"
)

// authenticate validates a Bearer credential and loads the current account.
func (s *Service) authenticate(ctx context.Context, authorization string) (userProfile, error) {
	scheme, raw, found := strings.Cut(authorization, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return userProfile{}, errCredentials
	}
	return s.profile(ctx, strings.TrimSpace(raw))
}
