package ingestion

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/httpapi"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/money"
)

type decodedRow struct {
	line             int
	accountCode      string
	sourceReference  string
	paymentReference string
	transactionDate  string
	description      string
	amount           string
	currency         string
}

type rowFields struct {
	FinancialAccountID string `json:"financial_account_id,omitempty" format:"uuid"`
	Line               int    `json:"line"`
	SourceReference    string `json:"source_reference,omitempty"`
	PaymentReference   string `json:"payment_reference,omitempty"`
	TransactionDate    string `json:"transaction_date" format:"date"`
	Description        string `json:"description"`
}

type stagedRow struct {
	rowFields
	MinorUnits  int64  `json:"minor_units"`
	Currency    string `json:"currency"`
	MinorDigits int    `json:"minor_digits"`
}

func parseDate(value string) (time.Time, bool) {
	date, err := time.Parse(time.DateOnly, value)
	return date, err == nil && len(value) == len(time.DateOnly) && date.Year() > 0
}

func validatePreviewRows(file *decodedUpload, start, end time.Time, accountsByCode map[string]string) ([]stagedRow, []httpapi.Issue) {
	var rows []stagedRow
	issues := slices.Clone(file.issues)
	for _, raw := range file.rows {
		before := len(issues)
		if reason := dateIssue(raw.transactionDate, start, end); reason != "" {
			issues = append(issues, rowIssue(raw.line, "transaction_date", reason))
		}

		currency := strings.ToUpper(raw.currency)
		scale, recognized := currencyMinorDigits[currency]
		var minorUnits int64
		if len(raw.currency) != 3 || !recognized {
			issues = append(issues, rowIssue(raw.line, "currency", "invalid_currency"))
		} else {
			var reason string
			minorUnits, reason = parseMinorUnits(raw.amount, scale)
			if reason != "" {
				issues = append(issues, rowIssue(raw.line, "amount", reason))
			}
		}

		var accountID string
		if accountsByCode != nil {
			code := strings.TrimSpace(raw.accountCode)
			accountID = accountsByCode[code]
			switch {
			case code == "":
				issues = append(issues, rowIssue(raw.line, financialAccountColumn, "missing_financial_account"))
			case accountID == "":
				issues = append(issues, httpapi.Issue{
					Line: raw.line, Field: financialAccountColumn, Reason: "unknown_financial_account", Value: code,
				})
			}
		}
		if len(issues) == before {
			rows = append(rows, stagedRow{
				Line:               raw.line,
				FinancialAccountID: accountID,
				SourceReference:    strings.TrimSpace(raw.sourceReference),
				PaymentReference:   strings.TrimSpace(raw.paymentReference),
				TransactionDate:    raw.transactionDate,
				Description:        raw.description,
				MinorUnits:         minorUnits,
				Currency:           currency,
				MinorDigits:        scale,
			})
		}
	}
	slices.SortStableFunc(issues, func(a, b httpapi.Issue) int { return cmp.Compare(a.Line, b.Line) })
	return rows, issues[:min(len(issues), maxRowIssues)]
}

func rowIssue(line int, field, reason string) httpapi.Issue {
	return httpapi.Issue{Line: line, Field: field, Reason: reason}
}

func dateIssue(value string, start, end time.Time) string {
	date, valid := parseDate(value)
	if !valid {
		return "invalid_date"
	}
	if date.Before(start) || date.After(end) {
		return "outside_period"
	}
	return ""
}

func coverageDateIssues(rows []stagedRow, start, end time.Time) []httpapi.Issue {
	var issues []httpapi.Issue
	for _, row := range rows {
		if reason := dateIssue(row.TransactionDate, start, end); reason != "" {
			issues = append(issues, rowIssue(row.Line, "transaction_date", reason))
			if len(issues) >= maxRowIssues {
				break
			}
		}
	}
	return issues
}

func parseMinorUnits(value string, scale int) (int64, string) {
	sign := ""
	if strings.HasPrefix(value, "-") || strings.HasPrefix(value, "+") {
		sign, value = value[:1], value[1:]
	}
	whole, fraction, hasDecimal := strings.Cut(value, ".")
	if !decimalDigits(whole) || (hasDecimal && !decimalDigits(fraction)) {
		return 0, "invalid_amount"
	}
	if len(fraction) > scale {
		return 0, "invalid_precision"
	}
	exact := sign + whole + fraction + strings.Repeat("0", scale-len(fraction))
	n, err := strconv.ParseInt(exact, 10, 64)
	if err != nil {
		return 0, "amount_out_of_range"
	}
	return n, ""
}

func decimalDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, digit := range s {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

func (r stagedRow) decimalAmount() string {
	return money.Decimal(r.MinorUnits, r.MinorDigits)
}
