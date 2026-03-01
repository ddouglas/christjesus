# Need Flow QA Checklist (2026-03-01)

## Scope
- Need onboarding flow from welcome -> location -> categories -> story -> documents -> review -> confirmation
- Includes document upload/metadata/delete and review submission acknowledgements

## Environment
- Workspace: `christjesus`
- Validation command: `command go build ./...`
- Result: ✅ pass

## Quick Checklist

### 1) Happy path: complete need flow to submit
- **Status:** ✅ PASS (code-path + wiring validation)
- **Evidence:**
  - Need route chain wired in `internal/server/server.go`
  - Review handler loads persisted need/story/categories/documents in `internal/server/onboarding.go`
  - Review POST sets status to `SUBMITTED` and redirects to confirmation in `internal/server/onboarding.go`

### 2) Edge case: continue from documents with no files uploaded
- **Status:** ✅ PASS
- **Expected:** Block continue unless user explicitly checks skip option
- **Evidence:**
  - Guard in handler: `if len(documents) == 0 && !skipDocuments` in `internal/server/onboarding.go`
  - UI skip checkbox and disabled continue button in `internal/server/templates/pages/onboarding/need/documents.html`

### 3) Edge case: submit review without required acknowledgements
- **Status:** ✅ PASS
- **Expected:** Redirect back to review with error message
- **Evidence:**
  - Server-side guard validates `agreeTerms` and `agreeVerification` in `internal/server/onboarding.go`
  - Redirect includes `error` query string back to review route

## Additional Notes
- Template math helper now supports mixed integer types (`int` and `int64`) to avoid runtime template failures on amount rendering.
- Document metadata update no longer mutates `uploaded_at` timestamps.

## Open/Deferred
- Browser-authenticated, click-through E2E (Cognito session) is recommended as a final smoke pass in a signed-in browser session.
