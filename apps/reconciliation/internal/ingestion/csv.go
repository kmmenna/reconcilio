package ingestion

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"slices"
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

const financialAccountColumn = "financial_account"

func decodeCSV(data []byte, mixed bool) ([]decodedRow, []httpapi.Issue) {
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return nil, []httpapi.Issue{rowIssue(0, "file", "invalid_encoding")}
	}

	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
	header, err := reader.Read()
	if err != nil || len(header) != len(csvColumns) && (!mixed || len(header) != len(csvColumns)+1) {
		return nil, []httpapi.Issue{rowIssue(1, "header", "invalid_header")}
	}
	headerLine, _ := reader.FieldPos(0)
	var columns [len(csvColumns)]int
	for i, name := range csvColumns {
		columns[i] = slices.Index(header, name)
		if columns[i] < 0 {
			return nil, []httpapi.Issue{rowIssue(headerLine, "header", "invalid_header")}
		}
	}

	accountColumn := slices.Index(header, financialAccountColumn)
	if len(header) > len(csvColumns) && accountColumn < 0 {
		return nil, []httpapi.Issue{rowIssue(headerLine, "header", "invalid_header")}
	}

	var rows []decodedRow
	var issues []httpapi.Issue
	for count := 0; ; count++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if parseError, ok := errors.AsType[*csv.ParseError](err); ok {
			field, reason := "row", "invalid_record"
			if errors.Is(err, csv.ErrFieldCount) {
				reason = "invalid_column_count"
				if mixed && accountColumn == len(header)-1 && len(record) == accountColumn {
					field, reason = financialAccountColumn, "missing_financial_account"
				}
			}
			issues = append(issues, rowIssue(parseError.StartLine, field, reason))
			// An unclosed quoted field can consume subsequent physical lines.
			if reason == "invalid_record" || len(issues) >= maxRowIssues {
				break
			}
			continue
		}
		if err != nil {
			return nil, []httpapi.Issue{rowIssue(0, "file", "invalid_record")}
		}

		line, _ := reader.FieldPos(0)
		if count >= maxPreviewRows {
			issues = append(issues, rowIssue(line, "row", "too_many_rows"))
			break
		}
		var accountCode string
		if accountColumn >= 0 {
			accountCode = record[accountColumn]
		}
		rows = append(rows, decodedRow{
			line:             line,
			accountCode:      accountCode,
			sourceReference:  record[columns[sourceReferenceColumn]],
			paymentReference: record[columns[paymentReferenceColumn]],
			transactionDate:  record[columns[dateColumn]],
			description:      record[columns[descriptionColumn]],
			amount:           record[columns[amountColumn]],
			currency:         record[columns[currencyColumn]],
		})
	}
	if len(issues) == 0 && len(rows) == 0 {
		issues = append(issues, rowIssue(0, "file", "no_rows"))
	}
	return rows, issues
}
