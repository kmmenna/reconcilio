package reconciliation

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/catalog"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/money"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/reconciliation/reconciliationdb"
)

type ProposalEvidence struct {
	ReferenceRelation      string `json:"reference_relation" enum:"missing,different,shared"`
	DaysApart              int32  `json:"days_apart" minimum:"0" maximum:"3"`
	InternalReferenceCount int64  `json:"internal_reference_count,omitzero"`
	ExternalReferenceCount int64  `json:"external_reference_count,omitzero"`
	Amount                 string `json:"amount"`
	Currency               string `json:"currency"`
}

type ReviewTransaction struct {
	FinancialAccount catalog.FinancialAccount `json:"financial_account"`
	Internal         Transaction              `json:"internal_transaction"`
}

type ReviewQueueEntry struct {
	ReviewTransaction
	PendingCount int64 `json:"pending_count"`
}

type ReviewGroup struct {
	ReviewTransaction
	Proposals []Proposal `json:"proposals"`
}

type ProposalResolution struct {
	Status           string         `json:"status" enum:"pending,accepted,rejected,invalidated"`
	ResolvedAt       *time.Time     `json:"resolved_at,omitempty"`
	Actor            *DecisionActor `json:"actor,omitempty"`
	ReconciliationID string         `json:"reconciliation_id,omitempty" format:"uuid"`
}

type Proposal struct {
	ID string `json:"id" format:"uuid"`
	ProposalResolution
	External  Transaction      `json:"external_transaction"`
	Evidence  ProposalEvidence `json:"evidence"`
	CreatedAt time.Time        `json:"created_at"`
}

type ProposalDetail struct {
	ReviewTransaction
	Proposal
}

// publishMatchProposals evaluates and publishes pairs in the account write transaction.
func publishMatchProposals(ctx context.Context, queries *reconciliationdb.Queries, accountID pgtype.UUID, jobID int64) error {
	pairs, err := queries.ListReviewCandidates(ctx, reconciliationdb.ListReviewCandidatesParams{AccountID: accountID})
	if err != nil {
		return fmt.Errorf("find candidates for proposal publication: %w", err)
	}
	for _, pair := range pairs {
		evidence := ProposalEvidence{
			ReferenceRelation: pair.ReferenceRelation, DaysApart: pair.DaysApart,
			InternalReferenceCount: pair.InternalReferenceCount, ExternalReferenceCount: pair.ExternalReferenceCount,
			Amount: money.Decimal(pair.AmountMinor, int(pair.MinorDigits)), Currency: pair.Currency,
		}
		if !evidence.valid() {
			return errors.New("invalid proposed match evidence")
		}
		data, err := json.Marshal(evidence)
		if err != nil {
			return fmt.Errorf("encode proposed match evidence: %w", err)
		}
		if err := queries.CreateMatchProposal(ctx, reconciliationdb.CreateMatchProposalParams{
			FinancialAccountID: accountID, InternalTransactionID: pair.InternalID, ExternalTransactionID: pair.ExternalID,
			Evidence: data, JobID: jobID,
		}); err != nil {
			return fmt.Errorf("record match proposal: %w", err)
		}
	}
	return nil
}

func (s *Service) ListReviewQueue(ctx context.Context) ([]ReviewQueueEntry, error) {
	rows, err := s.queries.ListReviewQueue(ctx)
	if err != nil {
		return nil, err
	}
	groups := make([]ReviewQueueEntry, 0, len(rows))
	for _, row := range rows {
		groups = append(groups, ReviewQueueEntry{
			ReviewTransaction: presentReviewTransaction(row.Transaction, row.FinancialAccount, row.SourceName),
			PendingCount:      row.PendingCount,
		})
	}
	return groups, nil
}

func (s *Service) GetReviewGroup(ctx context.Context, internalID pgtype.UUID) (ReviewGroup, error) {
	internal, err := s.queries.GetReviewTransaction(ctx, internalID)
	if err != nil {
		return ReviewGroup{}, err
	}
	rows, err := s.queries.ListMatchProposals(ctx, reconciliationdb.ListMatchProposalsParams{InternalID: internalID})
	if err != nil {
		return ReviewGroup{}, err
	}
	group := ReviewGroup{
		ReviewTransaction: presentReviewTransaction(internal.Transaction, internal.FinancialAccount, internal.SourceName),
		Proposals:         make([]Proposal, 0, len(rows)),
	}
	for _, row := range rows {
		proposal, err := presentMatchProposal(internal.Transaction, row)
		if err != nil {
			return ReviewGroup{}, err
		}
		group.Proposals = append(group.Proposals, proposal)
	}
	return group, nil
}

func (s *Service) GetMatchProposal(ctx context.Context, proposalID pgtype.UUID) (ProposalDetail, error) {
	rows, err := s.queries.ListMatchProposals(ctx, reconciliationdb.ListMatchProposalsParams{ProposalID: proposalID})
	if err != nil {
		return ProposalDetail{}, err
	}
	if len(rows) == 0 {
		return ProposalDetail{}, pgx.ErrNoRows
	}
	row := rows[0]
	internal, err := s.queries.GetReviewTransaction(ctx, row.MatchProposal.InternalTransactionID)
	if err != nil {
		return ProposalDetail{}, err
	}
	proposal, err := presentMatchProposal(internal.Transaction, row)
	if err != nil {
		return ProposalDetail{}, err
	}
	return ProposalDetail{
		ReviewTransaction: presentReviewTransaction(internal.Transaction, internal.FinancialAccount, internal.SourceName),
		Proposal:          proposal,
	}, nil
}

func presentReviewTransaction(item reconciliationdb.Transaction, account reconciliationdb.FinancialAccount, sourceName string) ReviewTransaction {
	return ReviewTransaction{FinancialAccount: presentAccount(account), Internal: presentTransaction(item, sourceName)}
}

func presentMatchProposal(internal reconciliationdb.Transaction, row reconciliationdb.ListMatchProposalsRow) (Proposal, error) {
	record := row.MatchProposal
	evidence, err := readProposalEvidence(record.Evidence, internal, row.Transaction)
	if err != nil {
		return Proposal{}, err
	}
	return Proposal{
		ID:                 record.ID.String(),
		ProposalResolution: presentProposalResolution(record),
		External:           presentTransaction(row.Transaction, row.SourceName),
		Evidence:           evidence,
		CreatedAt:          record.CreatedAt.Time,
	}, nil
}

func presentProposalResolution(record reconciliationdb.MatchProposal) ProposalResolution {
	resolution := ProposalResolution{Status: record.Status, ReconciliationID: record.ReconciliationID.String()}
	if record.ResolvedAt.Valid {
		resolution.ResolvedAt = new(record.ResolvedAt.Time)
		resolution.Actor = &DecisionActor{Kind: "engine"}
		if record.ResolvedBy.Valid {
			resolution.Actor = &DecisionActor{
				Kind: "analyst", UserID: record.ResolvedBy.String(), DisplayName: record.ResolvedByName.String,
			}
		}
	}
	return resolution
}

func readProposalEvidence(data []byte, internal, external reconciliationdb.Transaction) (ProposalEvidence, error) {
	var evidence ProposalEvidence
	if err := json.Unmarshal(data, &evidence, json.RejectUnknownMembers(true)); err != nil {
		return ProposalEvidence{}, fmt.Errorf("decode original proposal evidence: %w", err)
	}
	relation := "different"
	switch {
	case !internal.PaymentReference.Valid || !external.PaymentReference.Valid:
		relation = "missing"
	case internal.PaymentReference.String == external.PaymentReference.String:
		relation = "shared"
	}
	first, last := internal.TransactionDate.Time, external.TransactionDate.Time
	if first.After(last) {
		first, last = last, first
	}
	if !evidence.valid() || evidence.ReferenceRelation != relation ||
		!first.AddDate(0, 0, int(evidence.DaysApart)).Equal(last) ||
		internal.AmountMinor != external.AmountMinor || internal.MinorDigits != external.MinorDigits ||
		evidence.Amount != money.Decimal(internal.AmountMinor, int(internal.MinorDigits)) ||
		evidence.Currency != internal.Currency || evidence.Currency != external.Currency {
		return ProposalEvidence{}, errors.New("invalid original proposal evidence")
	}
	return evidence, nil
}

func (e ProposalEvidence) valid() bool {
	if e.DaysApart < 0 || e.DaysApart > 3 || e.Amount == "" || e.Currency == "" {
		return false
	}
	switch e.ReferenceRelation {
	case "missing", "different":
		return e.InternalReferenceCount == 0 && e.ExternalReferenceCount == 0
	case "shared":
		return e.InternalReferenceCount > 0 && e.ExternalReferenceCount > 0 &&
			(e.InternalReferenceCount > 1 || e.ExternalReferenceCount > 1)
	default:
		return false
	}
}
