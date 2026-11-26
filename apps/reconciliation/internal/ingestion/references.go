package ingestion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/httpapi"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/ingestion/ingestiondb"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/money"
)

type transactionValues struct {
	TransactionID    string `json:"transaction_id,omitempty" format:"uuid"`
	SourceReference  string `json:"source_reference"`
	PaymentReference string `json:"payment_reference,omitempty"`
	TransactionDate  string `json:"transaction_date" format:"date"`
	Amount           string `json:"amount"`
	Currency         string `json:"currency"`
	Description      string `json:"description"`
}

type importConflict struct {
	ID                 string            `json:"id"`
	FinancialAccountID string            `json:"financial_account_id" format:"uuid"`
	Line               int               `json:"line"`
	Fields             []string          `json:"fields"`
	Stored             transactionValues `json:"stored"`
	Incoming           transactionValues `json:"incoming"`
}

type accountAssessment struct {
	NewRows                 []stagedRow
	Conflicts               []importConflict
	AllHaveSourceReferences bool
}

func assessAccount(ctx context.Context, queries *ingestiondb.Queries, sourceID, accountID pgtype.UUID, rows []stagedRow) (accountAssessment, error) {
	assessment := accountAssessment{AllHaveSourceReferences: true}
	references := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.SourceReference == "" {
			assessment.AllHaveSourceReferences = false
		} else {
			references = append(references, row.SourceReference)
		}
	}
	storedByReference := make(map[string]transactionValues, len(references))
	if len(references) > 0 {
		records, err := queries.ListReferencedTransactions(ctx, ingestiondb.ListReferencedTransactionsParams{
			SourceID: sourceID, FinancialAccountID: accountID, SourceReferences: references,
		})
		if err != nil {
			return accountAssessment{}, fmt.Errorf("look up source references: %w", err)
		}
		for _, record := range records {
			storedByReference[record.SourceReference.String] = transactionValues{
				TransactionID:    record.ID.String(),
				SourceReference:  record.SourceReference.String,
				PaymentReference: record.PaymentReference.String,
				TransactionDate:  record.TransactionDate.Time.Format(time.DateOnly),
				Amount:           money.Decimal(record.AmountMinor, int(record.MinorDigits)),
				Currency:         record.Currency,
				Description:      record.Description,
			}
		}
	}

	newByReference := make(map[string]stagedRow, len(references))
	reported := make(map[int]bool)
	var issues []httpapi.Issue
	for _, row := range rows {
		if row.SourceReference == "" {
			assessment.NewRows = append(assessment.NewRows, row)
			continue
		}

		if stored, exists := storedByReference[row.SourceReference]; exists {
			conflict, err := compareTransaction(accountID, stored, row)
			if err != nil {
				return accountAssessment{}, err
			}
			if conflict.ID != "" {
				assessment.Conflicts = append(assessment.Conflicts, conflict)
			}
			continue
		}

		if first, exists := newByReference[row.SourceReference]; exists {
			normalized := row
			normalized.Line = first.Line
			if normalized != first {
				if !reported[first.Line] {
					issues = append(issues, duplicateReferenceIssue(first.Line))
					reported[first.Line] = true
				}
				issues = append(issues, duplicateReferenceIssue(row.Line))
				if len(issues) >= maxRowIssues {
					break
				}
			}
			continue
		}
		newByReference[row.SourceReference] = row
		assessment.NewRows = append(assessment.NewRows, row)
	}

	if len(issues) > 0 {
		return accountAssessment{}, importProblem("invalid_csv", "The CSV contains conflicting rows for a new Source Reference.", issues...)
	}
	return assessment, nil
}

func duplicateReferenceIssue(line int) httpapi.Issue {
	return httpapi.Issue{Line: line, Field: "source_reference", Reason: "conflicting_source_reference"}
}

func compareTransaction(accountID pgtype.UUID, stored transactionValues, row stagedRow) (importConflict, error) {
	incoming := transactionValues{
		SourceReference:  row.SourceReference,
		PaymentReference: row.PaymentReference,
		TransactionDate:  row.TransactionDate,
		Amount:           row.decimalAmount(),
		Currency:         row.Currency,
		Description:      row.Description,
	}
	var fields []string
	if comparableDecimal(stored.Amount) != comparableDecimal(incoming.Amount) {
		fields = append(fields, "amount")
	}
	if stored.Currency != incoming.Currency {
		fields = append(fields, "currency")
	}
	if stored.TransactionDate != incoming.TransactionDate {
		fields = append(fields, "transaction_date")
	}
	if stored.PaymentReference != incoming.PaymentReference {
		fields = append(fields, "payment_reference")
	}
	if len(fields) == 0 {
		return importConflict{}, nil
	}
	conflict := importConflict{
		FinancialAccountID: accountID.String(), Line: row.Line, Fields: fields, Stored: stored, Incoming: incoming,
	}
	discrepancy, err := json.Marshal(conflict)
	if err != nil {
		return importConflict{}, fmt.Errorf("encode source reference conflict: %w", err)
	}
	fingerprint := sha256.Sum256(discrepancy)
	conflict.ID = hex.EncodeToString(fingerprint[:])
	return conflict, nil
}

func comparableDecimal(value string) string {
	whole, fraction, _ := strings.Cut(value, ".")
	fraction = strings.TrimRight(fraction, "0")
	if fraction == "" {
		return whole
	}
	return whole + "." + fraction
}
