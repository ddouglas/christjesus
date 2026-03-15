package server

import (
	"strings"
	"testing"

	"christjesus/pkg/types"
)

func TestValidateNeedReviewMessageBody(t *testing.T) {
	t.Run("trims and accepts valid message", func(t *testing.T) {
		body, err := validateNeedReviewMessageBody("  hello world  ")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if body != "hello world" {
			t.Fatalf("unexpected body: got %q", body)
		}
	})

	t.Run("rejects empty", func(t *testing.T) {
		_, err := validateNeedReviewMessageBody("   ")
		if err == nil {
			t.Fatal("expected validation error for empty body")
		}
	})

	t.Run("rejects over max length", func(t *testing.T) {
		tooLong := strings.Repeat("a", types.NeedReviewMessageMaxChars+1)
		_, err := validateNeedReviewMessageBody(tooLong)
		if err == nil {
			t.Fatal("expected validation error for oversized body")
		}
	})
}

func TestLatestRejectedFeedback(t *testing.T) {
	needReasonNew := "Need still missing details"
	needNoteNew := "Please add a clearer summary"
	needReasonOld := "Old reason"
	needNoteOld := "Old note"
	docID := "doc-1"
	docReasonNew := "Document unreadable"
	docNoteNew := "Please upload a sharper image"
	docReasonOld := "Old document reason"

	actions := []*types.NeedModerationAction{
		{
			ActionType: types.NeedModerationActionTypeChangesRequested,
			Reason:     &needReasonNew,
			Note:       &needNoteNew,
		},
		{
			ActionType: types.NeedModerationActionTypeReviewRejected,
			Reason:     &needReasonOld,
			Note:       &needNoteOld,
		},
		{
			ActionType: types.NeedModerationActionTypeDocumentRejected,
			DocumentID: &docID,
			Reason:     &docReasonNew,
			Note:       &docNoteNew,
		},
		{
			ActionType: types.NeedModerationActionTypeDocumentRejected,
			DocumentID: &docID,
			Reason:     &docReasonOld,
		},
	}

	needReason, needNote, rejectedByDoc := latestRejectedFeedback(actions)
	if needReason != needReasonNew {
		t.Fatalf("unexpected need reason: got %q want %q", needReason, needReasonNew)
	}
	if needNote != needNoteNew {
		t.Fatalf("unexpected need note: got %q want %q", needNote, needNoteNew)
	}

	feedback, ok := rejectedByDoc[docID]
	if !ok {
		t.Fatalf("expected rejected feedback for doc %q", docID)
	}
	if feedback.reason != docReasonNew {
		t.Fatalf("unexpected doc reason: got %q want %q", feedback.reason, docReasonNew)
	}
	if feedback.note != docNoteNew {
		t.Fatalf("unexpected doc note: got %q want %q", feedback.note, docNoteNew)
	}
}
