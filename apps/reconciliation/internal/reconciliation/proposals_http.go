package reconciliation

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"uuid"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/httpapi"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/identity"
)

type reviewInput struct {
	Authorization string `header:"Authorization"`
}

type reviewOutput[T any] struct {
	CacheControl string `header:"Cache-Control"`
	Body         T
}

func (s *Service) registerReviewHTTP(api huma.API, identities *identity.Service, bearer []map[string][]string) {
	huma.Register(api, huma.Operation{
		OperationID: "list-review-queue",
		Method:      http.MethodGet,
		Path:        "/api/review-queue",
		Summary:     "List internal transactions with pending match proposals",
		Errors:      []int{http.StatusUnauthorized},
		Security:    bearer,
	}, func(ctx context.Context, in *reviewInput) (*reviewOutput[[]ReviewQueueEntry], error) {
		if _, err := identities.RequireUser(ctx, in.Authorization); err != nil {
			return nil, err
		}
		rows, err := s.ListReviewQueue(ctx)
		if err != nil {
			return nil, reviewUnavailable(ctx, err)
		}
		return &reviewOutput[[]ReviewQueueEntry]{CacheControl: "no-store", Body: rows}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "list-transaction-match-proposals",
		Method:      http.MethodGet,
		Path:        "/api/transactions/{id}/match-proposals",
		Summary:     "List pending alternatives for an internal transaction",
		Errors:      []int{http.StatusUnauthorized, http.StatusNotFound},
		Security:    bearer,
	}, func(ctx context.Context, in *detailInput) (*reviewOutput[ReviewGroup], error) {
		if _, err := identities.RequireUser(ctx, in.Authorization); err != nil {
			return nil, err
		}
		id, err := uuid.Parse(in.ID)
		if err != nil {
			return nil, reviewTransactionNotFound()
		}
		group, err := s.GetReviewGroup(ctx, pgtype.UUID{Bytes: id, Valid: true})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, reviewTransactionNotFound()
		}
		if err != nil {
			return nil, reviewUnavailable(ctx, err)
		}
		return &reviewOutput[ReviewGroup]{CacheControl: "no-store", Body: group}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "get-match-proposal",
		Method:      http.MethodGet,
		Path:        "/api/match-proposals/{id}",
		Summary:     "Read a match proposal and its original evidence and resolution",
		Errors:      []int{http.StatusUnauthorized, http.StatusNotFound},
		Security:    bearer,
	}, func(ctx context.Context, in *detailInput) (*reviewOutput[ProposalDetail], error) {
		if _, err := identities.RequireUser(ctx, in.Authorization); err != nil {
			return nil, err
		}
		id, err := uuid.Parse(in.ID)
		if err != nil {
			return nil, proposalNotFound()
		}
		detail, err := s.GetMatchProposal(ctx, pgtype.UUID{Bytes: id, Valid: true})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, proposalNotFound()
		}
		if err != nil {
			return nil, reviewUnavailable(ctx, err)
		}
		return &reviewOutput[ProposalDetail]{CacheControl: "no-store", Body: detail}, nil
	})
}

func reviewTransactionNotFound() *httpapi.Problem {
	return httpapi.NewProblem(http.StatusNotFound, "review_transaction_not_found", "The internal transaction could not be found.")
}

func proposalNotFound() *httpapi.Problem {
	return httpapi.NewProblem(http.StatusNotFound, "match_proposal_not_found", "The match proposal could not be found.")
}

func reviewUnavailable(ctx context.Context, err error) *httpapi.Problem {
	slog.ErrorContext(ctx, "Could not process review request", "error", err)
	return httpapi.NewProblem(http.StatusInternalServerError, "review_unavailable", "The review queue is temporarily unavailable.")
}

type decisionInput struct {
	Authorization string `header:"Authorization"`
	ID            string `path:"id"`
	Body          *decisionBody
}

type decisionBody struct {
	RankingEvaluationID string `json:"ranking_evaluation_id,omitempty" format:"uuid"`
}

func (s *Service) registerDecisionHTTP(api huma.API, identities *identity.Service, bearer []map[string][]string) {
	for _, action := range []struct {
		name, result, summary string
	}{
		{name: "confirm", result: "accepted", summary: "Confirm a match proposal"},
		{name: "reject", result: "rejected", summary: "Reject a match proposal"},
	} {
		huma.Register(api, huma.Operation{
			OperationID: action.name + "-match-proposal",
			Method:      http.MethodPost,
			Path:        "/api/match-proposals/{id}/" + action.name,
			Summary:     action.summary,
			Errors:      []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity},
			Security:    bearer,
		}, func(ctx context.Context, in *decisionInput) (*reviewOutput[ProposalResolution], error) {
			analystID, err := identities.RequireUser(ctx, in.Authorization)
			if err != nil {
				return nil, err
			}
			id, err := uuid.Parse(in.ID)
			if err != nil {
				return nil, proposalNotFound()
			}
			var rankingEvaluationID string
			if in.Body != nil {
				rankingEvaluationID = in.Body.RankingEvaluationID
			}
			result, err := s.decideMatchProposal(ctx, pgtype.UUID{Bytes: id, Valid: true}, analystID, action.result, rankingEvaluationID)
			switch {
			case errors.Is(err, pgx.ErrNoRows):
				return nil, proposalNotFound()
			case errors.Is(err, errProposalConflict):
				return nil, httpapi.NewProblem(http.StatusConflict, "match_proposal_conflict", "The proposal changed. Refresh the remaining options.")
			case errors.Is(err, errInvalidEvaluation):
				return nil, httpapi.NewProblem(http.StatusUnprocessableEntity, "invalid_ranking_evaluation", "The ranking evaluation does not cover the proposal.")
			case err != nil:
				return nil, reviewUnavailable(ctx, err)
			default:
				return &reviewOutput[ProposalResolution]{CacheControl: "no-store", Body: result}, nil
			}
		})
	}
}
