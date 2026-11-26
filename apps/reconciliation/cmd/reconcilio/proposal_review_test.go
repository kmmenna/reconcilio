package main

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/httpapi"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/reconciliation"
)

func verifyProposalReview(t *testing.T, accountID string, request func(method, path string) *httptest.ResponseRecorder, confirmCSV func(side, data string)) {
	t.Helper()

	readGroup := func(t *testing.T, internalID string) reconciliation.ReviewGroup {
		t.Helper()
		var group reconciliation.ReviewGroup
		readResponse(t, request(http.MethodGet, "/api/transactions/"+internalID+"/match-proposals"), http.StatusOK, &group)
		return group
	}
	readQueue := func(t *testing.T) map[string]reconciliation.ReviewQueueEntry {
		t.Helper()
		var queue []reconciliation.ReviewQueueEntry
		readResponse(t, request(http.MethodGet, "/api/review-queue"), http.StatusOK, &queue)
		groups := make(map[string]reconciliation.ReviewQueueEntry, len(queue))
		for _, group := range queue {
			groups[group.Internal.SourceReference] = group
		}
		return groups
	}

	queue := readQueue(t)
	if queue["ERP-002"].PendingCount != 2 || queue["ERP-LATE"].PendingCount != 1 ||
		queue["ERP-SIGN"].PendingCount != 0 || queue["ERP-CURRENCY"].PendingCount != 0 || queue["ERP-003"].PendingCount != 0 {
		t.Fatalf("unexpected groups for ambiguous pairs: %+v", queue)
	}
	erpID := queue["ERP-002"].Internal.ID
	alternatives := readGroup(t, erpID)
	references := map[string]string{}
	for _, proposal := range alternatives.Proposals {
		references[proposal.External.SourceReference] = proposal.External.Description
		if proposal.Evidence.ReferenceRelation != "missing" || proposal.Evidence.DaysApart != 1 ||
			proposal.Evidence.Amount != "-250.00" || proposal.Evidence.Currency != "BRL" {
			t.Fatalf("unexpected original proposal evidence: %+v", proposal)
		}
	}
	if len(alternatives.Proposals) != 2 || references["CARD-002"] != "CANVA* I045" || references["CARD-003"] != "ADOBE* CLOUD" {
		t.Fatalf("unexpected alternatives for ERP-002: %+v", alternatives.Proposals)
	}

	if alternatives.Proposals[0].External.ID >= alternatives.Proposals[1].External.ID {
		t.Fatalf("external Transaction IDs should break ties: %+v", alternatives.Proposals)
	}

	confirmCSV("internal", csvHeader+"ERP-BOUNDARY,,2026-11-22,-250.00,BRL,Another possible match\n")
	boundaryID := readQueue(t)["ERP-BOUNDARY"].Internal.ID
	boundary := readGroup(t, boundaryID)
	if len(boundary.Proposals) != 2 || boundary.Proposals[0].Evidence.DaysApart != 3 || boundary.Proposals[1].Evidence.DaysApart != 3 {
		t.Fatalf("the negative three-day boundary should include both alternatives: %+v", boundary)
	}

	t.Run("preserves competing proposals after simultaneous decisions", func(t *testing.T) {
		var before [2]reconciliation.ProposalDetail
		var proposalPaths [2]string
		for i, proposal := range alternatives.Proposals {
			proposalPaths[i] = "/api/match-proposals/" + proposal.ID
			readResponse(t, request(http.MethodGet, proposalPaths[i]), http.StatusOK, &before[i])
		}
		var decisions [2]*httptest.ResponseRecorder
		var confirmations sync.WaitGroup
		startTogether := make(chan struct{})
		for i, path := range proposalPaths {
			confirmations.Go(func() {
				<-startTogether
				decisions[i] = request(http.MethodPost, path+"/confirm")
			})
		}
		close(startTogether)
		confirmations.Wait()
		winner, loser := 0, 1
		if decisions[1].Code == http.StatusOK {
			winner, loser = loser, winner
		}
		var accepted reconciliation.ProposalResolution
		readResponse(t, decisions[winner], http.StatusOK, &accepted)
		var conflict httpapi.Problem
		readResponse(t, decisions[loser], http.StatusConflict, &conflict)
		if conflict.Code != "match_proposal_conflict" || accepted.Status != "accepted" ||
			accepted.ReconciliationID == "" || accepted.Actor == nil || accepted.Actor.Kind != "analyst" ||
			accepted.Actor.UserID == "" || accepted.ResolvedAt == nil {
			t.Fatalf("unexpected outcomes: accepted=%+v conflict=%+v", accepted, conflict)
		}

		retried := request(http.MethodPost, proposalPaths[winner]+"/confirm")
		if retried.Code != http.StatusOK || retried.Body.String() != decisions[winner].Body.String() {
			t.Fatalf("retry changed the recorded outcome: %d %s", retried.Code, retried.Body)
		}
		readResponse(t, request(http.MethodPost, proposalPaths[winner]+"/reject"), http.StatusConflict, &conflict)
		for i, proposalPath := range proposalPaths {
			var detail reconciliation.ProposalDetail
			readResponse(t, request(http.MethodGet, proposalPath), http.StatusOK, &detail)
			wantStatus := "invalidated"
			if i == winner {
				wantStatus = "accepted"
			}
			if detail.Status != wantStatus || detail.Evidence != before[i].Evidence || detail.CreatedAt != before[i].CreatedAt ||
				detail.ReconciliationID != accepted.ReconciliationID || detail.Actor == nil || *detail.Actor != *accepted.Actor {
				t.Fatalf("proposal detail after competing confirmation: %+v", detail)
			}
		}
		var group reconciliation.ReviewGroup
		readResponse(t, request(http.MethodGet, "/api/transactions/"+erpID+"/match-proposals"), http.StatusOK, &group)
		if len(group.Proposals) != 0 {
			t.Fatalf("resolved group remains in review: %+v", group)
		}
		for _, proposal := range boundary.Proposals {
			if proposal.External.ID != before[winner].External.ID {
				continue
			}
			var detail reconciliation.ProposalDetail
			readResponse(t, request(http.MethodGet, "/api/match-proposals/"+proposal.ID), http.StatusOK, &detail)
			if detail.Status != "invalidated" || detail.ReconciliationID != accepted.ReconciliationID {
				t.Fatalf("the same external Transaction remained available in another review group: %+v", detail)
			}
		}
		if remaining := readGroup(t, boundaryID); len(remaining.Proposals) != 1 {
			t.Fatalf("the unrelated pair should remain eligible: %+v", remaining)
		}
		var recorded reconciliation.Detail
		readResponse(t, request(http.MethodGet, "/api/reconciliations/"+accepted.ReconciliationID), http.StatusOK, &recorded)
		if recorded.MatchProposalID != before[winner].ID || recorded.ProposalEvidence == nil || *recorded.ProposalEvidence != before[winner].Evidence {
			t.Fatalf("the manual Reconciliation lost its original proposal evidence: %+v", recorded)
		}

		var reconciliations []reconciliation.Summary
		readResponse(t, request(http.MethodGet, "/api/reconciliations?financial_account_id="+accountID), http.StatusOK, &reconciliations)
		var manualCount int
		for _, item := range reconciliations {
			if item.Method == "manual" && item.ID == accepted.ReconciliationID {
				manualCount++
			}
		}
		if len(reconciliations) != 3 || manualCount != 1 {
			t.Fatalf("expected exactly one new reconciliation: %+v", reconciliations)
		}
	})

	t.Run("keeps original evidence and excludes a rejected pair from later runs", func(t *testing.T) {
		internalID := queue["ERP-LATE"].Internal.ID
		group := readGroup(t, internalID)
		original := group.Proposals[0]
		if original.External.SourceReference != "CARD-LATE" || original.Evidence.ReferenceRelation != "shared" ||
			original.Evidence.InternalReferenceCount != 2 || original.Evidence.ExternalReferenceCount != 2 {
			t.Fatalf("unexpected original shared reference evidence: %+v", original)
		}
		path := "/api/match-proposals/" + original.ID
		confirmCSV("external", csvHeader+
			"CARD-OTHER,DEMO-PAY-001,2026-11-16,-90.00,BRL,Increases the reference count\n"+
			"CARD-THREE,UNRELATED,2026-11-19,-89.90,BRL,Three days after\n"+
			"CARD-NEAR,,2026-11-17,-89.90,BRL,One day after\n"+
			"CARD-FOUR,,2026-11-20,-89.90,BRL,Four days after\n")
		group = readGroup(t, internalID)
		if len(group.Proposals) != 3 || group.Proposals[0].ID != original.ID || group.Proposals[0].Evidence != original.Evidence ||
			group.Proposals[1].External.SourceReference != "CARD-NEAR" || group.Proposals[2].External.SourceReference != "CARD-THREE" ||
			group.Proposals[2].Evidence.ReferenceRelation != "different" || group.Proposals[2].Evidence.DaysApart != 3 {
			t.Fatalf("unexpected alternatives after reprocessing: %+v", group)
		}
		decision := request(http.MethodPost, path+"/reject")
		var rejected reconciliation.ProposalResolution
		readResponse(t, decision, http.StatusOK, &rejected)
		if rejected.Status != "rejected" || rejected.ReconciliationID != "" || rejected.Actor == nil || rejected.ResolvedAt == nil {
			t.Fatalf("unexpected rejection: %+v", rejected)
		}
		retry := request(http.MethodPost, path+"/reject")
		if retry.Code != http.StatusOK || retry.Body.String() != decision.Body.String() {
			t.Fatalf("retry changed the rejection: %d %s", retry.Code, retry.Body)
		}
		var conflict httpapi.Problem
		readResponse(t, request(http.MethodPost, path+"/confirm"), http.StatusConflict, &conflict)

		confirmCSV("internal", csvHeader+"ERP-OTHER,NEW-REF,2026-11-17,-89.90,BRL,The rejected external Transaction may match this\n")
		group = readGroup(t, internalID)
		if len(group.Proposals) != 2 || group.Proposals[0].ID == original.ID || group.Proposals[1].ID == original.ID {
			t.Fatalf("the rejected pair reappeared or another pair was lost: %+v", group)
		}
		other := readGroup(t, readQueue(t)["ERP-OTHER"].Internal.ID)
		var stillEligible bool
		for _, proposal := range other.Proposals {
			if proposal.External.ID == original.External.ID {
				stillEligible = true
			}
		}
		if !stillEligible {
			t.Fatal("the rejected external Transaction should be eligible for other pairs")
		}
		var detail reconciliation.ProposalDetail
		readResponse(t, request(http.MethodGet, path), http.StatusOK, &detail)
		if detail.Status != "rejected" || detail.Evidence != original.Evidence || detail.Actor == nil || *detail.Actor != *rejected.Actor {
			t.Fatalf("the rejection history changed: %+v", detail)
		}
	})

}
