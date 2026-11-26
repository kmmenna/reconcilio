package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type openTransactionResponse struct {
	ID                  string `json:"id"`
	SourceReference     string `json:"source_reference"`
	Side                string `json:"side"`
	Amount              string `json:"amount"`
	Currency            string `json:"currency"`
	Description         string `json:"description"`
	HasPendingProposals bool   `json:"has_pending_match_proposals"`
	Discrepancy         *struct {
		PaymentReference string `json:"payment_reference"`
		Amount           string `json:"amount"`
		Currency         string `json:"currency"`
	} `json:"amount_discrepancy"`
}

func readOpenTransactions(t *testing.T, response *httptest.ResponseRecorder) map[string]openTransactionResponse {
	t.Helper()
	var transactions []openTransactionResponse
	readResponse(t, response, http.StatusOK, &transactions)
	byReference := make(map[string]openTransactionResponse, len(transactions))
	for _, item := range transactions {
		byReference[item.SourceReference] = item
	}
	return byReference
}

func verifyDemoOpenTransactions(t *testing.T, get func(path string) *httptest.ResponseRecorder) {
	t.Helper()
	open := readOpenTransactions(t, get("/api/transactions?status=open"))
	for _, reference := range []string{"ERP-001", "CARD-001", "ERP-EXACT", "CARD-EXACT"} {
		if _, exists := open[reference]; exists {
			t.Errorf("reconciled transaction %s is still open", reference)
		}
	}
	for _, want := range []struct{ reference, side, amount, description string }{
		{"ERP-003", "internal", "-120.00", "Office supplies order"},
		{"CARD-004", "external", "-119.00", "OFFICE SUPPLIES STORE"},
	} {
		item := open[want.reference]
		if item.ID == "" || item.Side != want.side || item.Amount != want.amount || item.Currency != "BRL" ||
			item.Description != want.description || item.HasPendingProposals || item.Discrepancy == nil ||
			item.Discrepancy.PaymentReference != "DEMO-PAY-003" || item.Discrepancy.Amount != "1.00" || item.Discrepancy.Currency != "BRL" {
			t.Errorf("%s should be open with a BRL 1.00 discrepancy and no pending proposals: %+v", want.reference, item)
		}
	}
	for _, reference := range []string{"ERP-002", "CARD-002", "CARD-003"} {
		if item := open[reference]; item.ID == "" || !item.HasPendingProposals || item.Discrepancy != nil {
			t.Errorf("%s should be open with pending proposals and no discrepancy: %+v", reference, item)
		}
	}
	for _, reference := range []string{"ERP-CURRENCY", "CARD-CURRENCY"} {
		if item := open[reference]; item.ID == "" || item.Discrepancy != nil {
			t.Errorf("%s should have no discrepancy across currencies: %+v", reference, item)
		}
	}
	for _, reference := range []string{"ERP-SIGN", "CARD-SIGN"} {
		if item := open[reference]; item.Discrepancy == nil || item.Discrepancy.Amount != "15.000" || item.Discrepancy.Currency != "KWD" {
			t.Errorf("%s should show the KWD 15.000 difference between opposite-signed values: %+v", reference, item)
		}
	}
}

func verifyOpenTransactionEdges(t *testing.T, get func(path string) *httptest.ResponseRecorder, confirmCSV func(side, data string)) {
	t.Helper()
	confirmCSV("internal", csvHeader+
		"ERP-WIDE,PAY-WIDE,2026-11-16,-9223372036854775.808,IQD,Largest negative amount\n"+
		"ERP-LATE-EXACT,PAY-EXACT,2026-11-17,1.000,IQD,Late internal reference\n")
	confirmCSV("external", csvHeader+
		"CARD-WIDE,PAY-WIDE,2026-11-16,9223372036854775.807,IQD,Largest positive amount\n"+
		"CARD-LATE-EXACT,PAY-EXACT,2026-11-17,2.000,IQD,Late external reference\n"+
		"CARD-ALT,,2026-11-21,-120.00,BRL,Another eligible office supply record\n")

	open := readOpenTransactions(t, get("/api/transactions?status=open"))
	for _, reference := range []string{"ERP-WIDE", "CARD-WIDE"} {
		if item := open[reference]; item.Discrepancy == nil || item.Discrepancy.Amount != "18446744073709551.615" ||
			item.Discrepancy.Currency != "IQD" || item.Discrepancy.PaymentReference != "PAY-WIDE" || item.HasPendingProposals {
			t.Errorf("%s should preserve the exact money difference beyond int64 minor units: %+v", reference, item)
		}
	}
	for _, reference := range []string{"ERP-LATE-EXACT", "CARD-LATE-EXACT"} {
		if item := open[reference]; item.ID == "" || item.Discrepancy != nil {
			t.Errorf("%s should remain open without a discrepancy because the reference is reused by reconciled Transactions: %+v", reference, item)
		}
	}
	if item := open["ERP-003"]; !item.HasPendingProposals || item.Discrepancy == nil || item.Discrepancy.Amount != "1.00" {
		t.Errorf("still show a discrepancy when ERP-003 has an unrelated pending proposal: %+v", item)
	}
	if item := open["CARD-004"]; item.HasPendingProposals || item.Discrepancy == nil || item.Discrepancy.Amount != "1.00" {
		t.Errorf("CARD-004 should have its own pending indicator and the shared discrepancy: %+v", item)
	}
}
