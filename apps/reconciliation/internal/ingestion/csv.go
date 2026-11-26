package ingestion

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/httpapi"
)

const (
	maxPreviewRows = 10_000
	maxRowIssues   = 100
)

const (
	sourceReferenceColumn = iota
	paymentReferenceColumn
	dateColumn
	amountColumn
	currencyColumn
	descriptionColumn
)

var csvColumns = [...]string{
	sourceReferenceColumn:  "source_reference",
	paymentReferenceColumn: "payment_reference",
	dateColumn:             "transaction_date",
	amountColumn:           "amount",
	currencyColumn:         "currency",
	descriptionColumn:      "description",
}

type rowFields struct {
	Line             int    `json:"line"`
	SourceReference  string `json:"source_reference,omitempty"`
	PaymentReference string `json:"payment_reference,omitempty"`
	TransactionDate  string `json:"transaction_date" format:"date"`
	Description      string `json:"description"`
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

func parseCSV(data []byte, start, end time.Time) ([]stagedRow, []httpapi.Issue) {
	issue := func(line int, field, reason string) httpapi.Issue {
		return httpapi.Issue{Line: line, Field: field, Reason: reason}
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return nil, []httpapi.Issue{issue(0, "file", "invalid_encoding")}
	}

	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
	reader.FieldsPerRecord = len(csvColumns)
	header, err := reader.Read()
	if err != nil {
		return nil, []httpapi.Issue{issue(1, "header", "invalid_header")}
	}
	headerLine, _ := reader.FieldPos(0)
	var columns [len(csvColumns)]int
	for i, name := range csvColumns {
		columns[i] = slices.Index(header, name)
		if columns[i] < 0 {
			return nil, []httpapi.Issue{issue(headerLine, "header", "invalid_header")}
		}
	}

	var rows []stagedRow
	var issues []httpapi.Issue
	for count := 0; ; count++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if parseError, ok := errors.AsType[*csv.ParseError](err); ok {
			reason := "invalid_record"
			if errors.Is(err, csv.ErrFieldCount) {
				reason = "invalid_column_count"
			}
			issues = append(issues, issue(parseError.StartLine, "row", reason))
			// An unclosed quoted field can consume subsequent physical lines.
			if reason == "invalid_record" || len(issues) >= maxRowIssues {
				break
			}
			continue
		}
		if err != nil {
			return nil, []httpapi.Issue{issue(0, "file", "invalid_record")}
		}

		line, _ := reader.FieldPos(0)
		if count >= maxPreviewRows {
			issues = append(issues, issue(line, "row", "too_many_rows"))
			break
		}
		before := len(issues)
		dateValue := record[columns[dateColumn]]
		date, validDate := parseDate(dateValue)
		switch {
		case !validDate:
			issues = append(issues, issue(line, "transaction_date", "invalid_date"))
		case date.Before(start) || date.After(end):
			issues = append(issues, issue(line, "transaction_date", "outside_period"))
		}

		currencyCode := record[columns[currencyColumn]]
		currency := strings.ToUpper(currencyCode)
		scale, recognized := currencyMinorDigits[currency]
		var minorUnits int64
		if len(currencyCode) != 3 || !recognized {
			issues = append(issues, issue(line, "currency", "invalid_currency"))
		} else {
			var reason string
			minorUnits, reason = parseMinorUnits(record[columns[amountColumn]], scale)
			if reason != "" {
				issues = append(issues, issue(line, "amount", reason))
			}
		}

		if len(issues) >= maxRowIssues {
			break
		}
		if len(issues) == before {
			rows = append(rows, stagedRow{
				rowFields: rowFields{
					Line:             line,
					SourceReference:  strings.TrimSpace(record[columns[sourceReferenceColumn]]),
					PaymentReference: strings.TrimSpace(record[columns[paymentReferenceColumn]]),
					TransactionDate:  dateValue,
					Description:      record[columns[descriptionColumn]],
				},
				MinorUnits:  minorUnits,
				Currency:    currency,
				MinorDigits: scale,
			})
		}
	}
	if len(issues) == 0 && len(rows) == 0 {
		issues = append(issues, issue(0, "file", "no_rows"))
	}
	return rows, issues
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
	scale := r.MinorDigits
	value := strconv.FormatInt(r.MinorUnits, 10)
	if scale == 0 {
		return value
	}
	digits, negative := strings.CutPrefix(value, "-")
	digits = strings.Repeat("0", max(0, scale+1-len(digits))) + digits
	cut := len(digits) - scale
	amount := digits[:cut] + "." + digits[cut:]
	if negative {
		return "-" + amount
	}
	return amount
}
