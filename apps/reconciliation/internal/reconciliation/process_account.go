package reconciliation

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/money"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/reconciliation/reconciliationdb"
)

// ProcessAccount evaluates open Transactions and commits each matching phase independently.
func (s *Service) ProcessAccount(ctx context.Context, attempt *AccountAttempt) error {
	if err := s.confirmAutomaticMatches(ctx, attempt); err != nil {
		return err
	}
	evaluations, err := s.rankReviewGroups(ctx, attempt)
	if err != nil {
		return err
	}
	var discardedEvaluations int
	err = attempt.write(ctx, func(tx pgx.Tx) error {
		var err error
		discardedEvaluations, err = publishMatchProposals(ctx, s.queries.WithTx(tx), attempt, evaluations)
		return err
	})
	if err == nil && s.ranking.Observe != nil {
		for range discardedEvaluations {
			s.ranking.Observe(ctx, RankingEvent{
				Provider: s.ranking.Provider, Model: s.ranking.Model, FallbackReason: "group_changed",
			})
		}
	}
	return err
}

func (s *Service) confirmAutomaticMatches(ctx context.Context, attempt *AccountAttempt) error {
	accountID, jobID := attempt.accountID, attempt.jobID
	return attempt.write(ctx, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		// Count all account references before excluding Transactions that already have a member.
		pairs, err := queries.ListAutomaticPairs(ctx, accountID)
		if err != nil {
			return fmt.Errorf("find automatic reconciliation pairs: %w", err)
		}
		for _, pair := range pairs {
			evidence := MatchEvidence{
				PaymentReference: pair.PaymentReference.String, InternalReferenceCount: 1, ExternalReferenceCount: 1,
				Amount: money.Decimal(pair.AmountMinor, int(pair.MinorDigits)), Currency: pair.Currency,
			}
			data, err := json.Marshal(evidence)
			if err != nil {
				return fmt.Errorf("encode automatic match evidence: %w", err)
			}
			id, err := recordReconciliationPair(ctx, queries, reconciliationdb.CreateReconciliationParams{
				FinancialAccountID: accountID, Method: "automatic", Evidence: data,
				JobID: pgtype.Int8{Int64: jobID, Valid: true},
			}, pair.InternalID, pair.ExternalID)
			if err != nil {
				return err
			}
			if err := invalidateCompetingProposals(ctx, queries, id, pair.InternalID, pair.ExternalID, pgtype.Text{}); err != nil {
				return err
			}
		}
		return nil
	})
}
