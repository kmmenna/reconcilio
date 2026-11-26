package reconciliation

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/catalog"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/reconciliation/reconciliationdb"
)

var (
	errProposalConflict  = errors.New("match proposal is no longer eligible for this decision")
	errInvalidEvaluation = errors.New("ranking evaluation does not cover the match proposal")
)

func (s *Service) decideMatchProposal(ctx context.Context, id, analystID pgtype.UUID, decision, rankingEvaluationID string) (ProposalResolution, error) {
	initial, err := s.queries.GetMatchProposalRecord(ctx, id)
	if err != nil {
		return ProposalResolution{}, err
	}
	var resolution ProposalResolution
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := catalog.LockFinancialAccounts(ctx, tx, []pgtype.UUID{initial.FinancialAccountID}); err != nil {
			return err
		}
		queries := s.queries.WithTx(tx)
		proposal, err := queries.GetMatchProposalRecord(ctx, id)
		if err != nil {
			return err
		}
		switch proposal.Status {
		case decision:
			resolution = presentProposalResolution(proposal)
			return nil
		case "pending":
		default:
			return errProposalConflict
		}
		if rankingEvaluationID != "" {
			return errInvalidEvaluation
		}
		params := reconciliationdb.ResolveMatchProposalParams{ID: id, AnalystID: analystID, Status: decision}
		if decision == "accepted" {
			pairs, err := queries.ListReviewCandidates(ctx, reconciliationdb.ListReviewCandidatesParams{
				AccountID: proposal.FinancialAccountID, InternalID: proposal.InternalTransactionID, ExternalID: proposal.ExternalTransactionID,
			})
			if err != nil {
				return fmt.Errorf("check proposal eligibility: %w", err)
			}
			if len(pairs) == 0 {
				return errProposalConflict
			}
			params.ReconciliationID, err = recordReconciliationPair(ctx, queries, reconciliationdb.CreateReconciliationParams{
				FinancialAccountID: proposal.FinancialAccountID, Method: "manual", ConfirmedBy: analystID,
			}, proposal.InternalTransactionID, proposal.ExternalTransactionID)
			if err != nil {
				return err
			}
		}
		resolved, err := queries.ResolveMatchProposal(ctx, params)
		if err != nil {
			return fmt.Errorf("record match proposal decision: %w", err)
		}
		if resolved.ReconciliationID.Valid {
			if err := invalidateCompetingProposals(ctx, queries, resolved.ReconciliationID,
				resolved.InternalTransactionID, resolved.ExternalTransactionID, resolved.ResolvedByName); err != nil {
				return err
			}
		}
		resolution = presentProposalResolution(resolved)
		return nil
	})
	return resolution, err
}

func invalidateCompetingProposals(ctx context.Context, queries *reconciliationdb.Queries,
	reconciliationID, internalID, externalID pgtype.UUID, analystName pgtype.Text,
) error {
	err := queries.InvalidateCompetingMatchProposals(ctx, reconciliationdb.InvalidateCompetingMatchProposalsParams{
		ReconciliationID: reconciliationID, InternalID: internalID, ExternalID: externalID, AnalystName: analystName,
	})
	if err != nil {
		return fmt.Errorf("invalidate competing match proposals: %w", err)
	}
	return nil
}

func recordReconciliationPair(ctx context.Context, queries *reconciliationdb.Queries,
	params reconciliationdb.CreateReconciliationParams, internalID, externalID pgtype.UUID,
) (pgtype.UUID, error) {
	id, err := queries.CreateReconciliation(ctx, params)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("record reconciliation: %w", err)
	}
	if err := queries.InsertReconciliationPair(ctx, reconciliationdb.InsertReconciliationPairParams{
		ReconciliationID: id, InternalID: internalID, ExternalID: externalID,
	}); err != nil {
		return pgtype.UUID{}, fmt.Errorf("record reconciliation members: %w", err)
	}
	return id, nil
}
