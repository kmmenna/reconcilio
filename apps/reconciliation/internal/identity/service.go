package identity

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/identity/identitydb"
)

const (
	tokenLifetime = 30 * time.Minute
	tokenIssuer   = "reconcilio"
	tokenAudience = "reconcilio-api"
)

var errCredentials = errors.New("invalid credentials")

// Service owns local credentials and access tokens.
type Service struct {
	users     *identitydb.Queries
	key       []byte
	dummyHash string
}

func New(pool *pgxpool.Pool, key []byte) *Service {
	return &Service{
		users:     identitydb.New(pool),
		key:       key,
		dummyHash: hashPassword(rand.Text()),
	}
}

// BootstrapAdmin inserts the initial account and returns its password only to the winning caller.
func (s *Service) BootstrapAdmin(ctx context.Context) (password string, created bool, err error) {
	password = rand.Text()
	_, err = s.users.CreateBootstrapAdmin(ctx, hashPassword(password))
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("create initial administrator: %w", err)
	}
	return password, true, nil
}

func (s *Service) login(ctx context.Context, username, password string) (string, error) {
	credentials, err := s.users.FindCredentials(ctx, username)
	if errors.Is(err, pgx.ErrNoRows) {
		verifyPassword(password, s.dummyHash)
		return "", errCredentials
	}
	if err != nil {
		return "", fmt.Errorf("look up credentials: %w", err)
	}
	if !verifyPassword(password, credentials.PasswordHash) {
		return "", errCredentials
	}

	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    tokenIssuer,
		Subject:   credentials.ID,
		Audience:  jwt.ClaimStrings{tokenAudience},
		ExpiresAt: jwt.NewNumericDate(now.Add(tokenLifetime)),
		IssuedAt:  jwt.NewNumericDate(now),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.key)
}

func (s *Service) subject(raw string) (string, error) {
	claims := new(jwt.RegisteredClaims)
	_, err := jwt.ParseWithClaims(raw, claims, func(_ *jwt.Token) (any, error) {
		return s.key, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(tokenIssuer), jwt.WithAudience(tokenAudience), jwt.WithExpirationRequired())
	if err != nil {
		return "", errCredentials
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return "", errCredentials
	}
	return id.String(), nil
}

type userProfile struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	IsAdmin     bool   `json:"is_admin"`
}

func (s *Service) profile(ctx context.Context, token string) (userProfile, error) {
	id, err := s.subject(token)
	if err != nil {
		return userProfile{}, err
	}
	user, err := s.users.FindProfile(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return userProfile{}, errCredentials
	}
	if err != nil {
		return userProfile{}, fmt.Errorf("look up user profile: %w", err)
	}
	return userProfile(user), nil
}
