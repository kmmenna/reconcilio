package reconciliation

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/catalog"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/money"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/reconciliation/reconciliationdb"
)

type Service struct {
	pool    *pgxpool.Pool
	queries *reconciliationdb.Queries
	ranking RankingOptions
}

func New(pool *pgxpool.Pool) *Service {
	return NewWithRanking(pool, DefaultRankingOptions())
}

func NewWithRanking(pool *pgxpool.Pool, ranking RankingOptions) *Service {
	return &Service{pool: pool, queries: reconciliationdb.New(pool), ranking: ranking}
}

type MatchEvidence struct {
	PaymentReference       string `json:"payment_reference"`
	InternalReferenceCount int    `json:"internal_reference_count"`
	ExternalReferenceCount int    `json:"external_reference_count"`
	Amount                 string `json:"amount"`
	Currency               string `json:"currency"`
}

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

type Summary struct {
	ID               string                   `json:"id" format:"uuid"`
	FinancialAccount catalog.FinancialAccount `json:"financial_account"`
	Amount           string                   `json:"amount"`
	Currency         string                   `json:"currency"`
	ConfirmedAt      time.Time                `json:"confirmed_at"`
	Method           string                   `json:"method" enum:"automatic,manual"`
}

type Transaction struct {
	ID               string         `json:"id" format:"uuid"`
	Side             string         `json:"side" enum:"internal,external"`
	Source           catalog.Source `json:"source"`
	SourceReference  string         `json:"source_reference,omitempty"`
	PaymentReference string         `json:"payment_reference,omitempty"`
	TransactionDate  string         `json:"transaction_date" format:"date"`
	Amount           string         `json:"amount"`
	Currency         string         `json:"currency"`
	Description      string         `json:"description"`
}

type DecisionActor struct {
	Kind        string `json:"kind" enum:"engine,analyst"`
	UserID      string `json:"user_id,omitempty" format:"uuid"`
	DisplayName string `json:"display_name,omitempty"`
}

type Detail struct {
	Summary
	Actor             DecisionActor      `json:"actor"`
	Members           []Transaction      `json:"members"`
	Evidence          *MatchEvidence     `json:"evidence,omitempty"`
	MatchProposalID   string             `json:"match_proposal_id,omitempty" format:"uuid"`
	ProposalEvidence  *ProposalEvidence  `json:"proposal_evidence,omitempty"`
	RankingAssessment *RankingAssessment `json:"ranking_assessment,omitempty"`
}

func (s *Service) List(ctx context.Context, accountID pgtype.UUID) ([]Summary, error) {
	rows, err := s.queries.ListReconciliations(ctx, accountID)
	if err != nil {
		return nil, err
	}
	results := make([]Summary, 0, len(rows))
	for _, row := range rows {
		results = append(results, Summary{
			ID: row.ID.String(), Method: row.Method, ConfirmedAt: row.ConfirmedAt.Time,
			FinancialAccount: presentAccount(row.FinancialAccount),
			Amount:           money.Decimal(row.AmountMinor, int(row.MinorDigits)), Currency: row.Currency,
		})
	}
	return results, nil
}

func (s *Service) Get(ctx context.Context, id pgtype.UUID) (Detail, error) {
	row, err := s.queries.GetReconciliation(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	members, err := s.queries.ListReconciliationMembers(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	record := row.Reconciliation
	detail := Detail{
		Summary: Summary{ID: id.String(), Method: record.Method, ConfirmedAt: record.ConfirmedAt.Time,
			FinancialAccount: presentAccount(row.FinancialAccount)},
		Actor:   DecisionActor{Kind: "engine"},
		Members: make([]Transaction, 0, len(members)),
	}
	if record.ConfirmedBy.Valid {
		detail.Actor = DecisionActor{Kind: "analyst", UserID: record.ConfirmedBy.String(), DisplayName: row.AnalystName}
	}
	for _, member := range members {
		transaction := presentTransaction(member.Transaction, member.SourceName)
		if member.Side == "internal" {
			detail.Amount = transaction.Amount
			detail.Currency = transaction.Currency
		}
		detail.Members = append(detail.Members, transaction)
	}
	if record.Method == "automatic" {
		evidence, err := readAutomaticEvidence(record.Evidence, detail.Amount, detail.Currency)
		if err != nil {
			return Detail{}, err
		}
		detail.Evidence = &evidence
	} else {
		proposalID, err := s.queries.GetAcceptedMatchProposalID(ctx, id)
		if err != nil {
			return Detail{}, fmt.Errorf("find accepted proposal: %w", err)
		}
		proposal, err := s.GetMatchProposal(ctx, proposalID)
		if err != nil {
			return Detail{}, fmt.Errorf("load accepted proposal: %w", err)
		}
		detail.MatchProposalID = proposal.ID
		detail.ProposalEvidence = &proposal.Evidence
		detail.RankingAssessment = proposal.RankingAssessment
		detail.Actor = *proposal.Actor
	}
	return detail, nil
}

func readAutomaticEvidence(data []byte, amount, currency string) (MatchEvidence, error) {
	var evidence MatchEvidence
	if err := json.Unmarshal(data, &evidence); err != nil {
		return MatchEvidence{}, fmt.Errorf("decode automatic match evidence: %w", err)
	}
	if evidence.PaymentReference == "" || evidence.InternalReferenceCount != 1 || evidence.ExternalReferenceCount != 1 ||
		evidence.Amount != amount || evidence.Currency != currency {
		return MatchEvidence{}, errors.New("invalid automatic match evidence")
	}
	return evidence, nil
}

func presentAccount(row reconciliationdb.FinancialAccount) catalog.FinancialAccount {
	return catalog.FinancialAccount{ID: row.ID.String(), Code: row.Code, Name: row.Name}
}

func presentTransaction(item reconciliationdb.Transaction, sourceName string) Transaction {
	return Transaction{
		ID: item.ID.String(), Side: item.SourceSide,
		Source:          catalog.Source{ID: item.SourceID.String(), Name: sourceName, Side: item.SourceSide},
		SourceReference: item.SourceReference.String, PaymentReference: item.PaymentReference.String,
		TransactionDate: item.TransactionDate.Time.Format(time.DateOnly),
		Amount:          money.Decimal(item.AmountMinor, int(item.MinorDigits)), Currency: item.Currency, Description: item.Description,
	}
}
