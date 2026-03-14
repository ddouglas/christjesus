package server

import (
	"testing"

	"christjesus/pkg/types"
)

func TestLatestDocumentStatuses_PrefersMostRecentModerationAction(t *testing.T) {
	docID := "doc-1"
	actions := []*types.NeedModerationAction{
		{ActionType: types.NeedModerationActionTypeDocumentRejected, DocumentID: &docID},
		{ActionType: types.NeedModerationActionTypeDocumentVerified, DocumentID: &docID},
	}

	statuses := latestDocumentStatuses(actions)
	if got, want := statuses[docID], "Rejected"; got != want {
		t.Fatalf("unexpected status for %s: got %q, want %q", docID, got, want)
	}
}

func TestLatestDocumentStatuses_IgnoresOlderActionsAfterFirstMatch(t *testing.T) {
	docID := "doc-2"
	actions := []*types.NeedModerationAction{
		{ActionType: types.NeedModerationActionTypeDocumentVerified, DocumentID: &docID},
		{ActionType: types.NeedModerationActionTypeDocumentRejected, DocumentID: &docID},
		{ActionType: types.NeedModerationActionTypeDocumentVerified, DocumentID: &docID},
	}

	statuses := latestDocumentStatuses(actions)
	if got, want := statuses[docID], "Verified"; got != want {
		t.Fatalf("unexpected status for %s: got %q, want %q", docID, got, want)
	}
}

func TestLatestDocumentStatuses_IgnoresNonDocumentActions(t *testing.T) {
	docID := "doc-3"
	actions := []*types.NeedModerationAction{
		{ActionType: types.NeedModerationActionTypeReviewApproved, DocumentID: &docID},
		{ActionType: types.NeedModerationActionTypeReviewRejected, DocumentID: &docID},
	}

	statuses := latestDocumentStatuses(actions)
	if _, exists := statuses[docID]; exists {
		t.Fatalf("expected no status for %s from non-document actions", docID)
	}
}
