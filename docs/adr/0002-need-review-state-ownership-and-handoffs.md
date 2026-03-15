# ADR 0002: Need Review State Ownership and Handoffs

## Status
Proposed

## Date
2026-03-15

## Context
We need a review workflow that prevents conflicting edits between need owners and reviewers without introducing a full versioning system.

The core concern is avoiding lost work when one party starts acting on a need while the other party is still editing.

## Decision
Adopt explicit state ownership and handoff rules for need review states.

### State ownership model
- Owner-owned states: `SUBMITTED`, `READY_FOR_REVIEW`, `CHANGES_REQUESTED`
- Reviewer-owned state: `UNDER_REVIEW`
- Terminal outcome states: `APPROVED`, `REJECTED`

### Primary state path
- `SUBMITTED -> READY_FOR_REVIEW -> UNDER_REVIEW -> CHANGES_REQUESTED -> APPROVED/REJECTED`

### Handoff semantics
- Need owner moves a need from `SUBMITTED` to `READY_FOR_REVIEW` to signal it is ready and locked from owner edits.
- Need owner may pull back `READY_FOR_REVIEW -> SUBMITTED` before a reviewer accepts it.
- Reviewer accepts review with explicit action `READY_FOR_REVIEW -> UNDER_REVIEW`.
- While `UNDER_REVIEW`, owner edits are blocked.
- Reviewer may complete review by moving to `CHANGES_REQUESTED`, `APPROVED`, or `REJECTED`.

### CHANGES_REQUESTED behavior (current policy draft)
- Ownership: need owner
- Editing: owner edits allowed
- Reviewer moderation actions are paused until owner resubmits
- Owner must explicitly resubmit by moving to `READY_FOR_REVIEW` after making updates
- Transition should not happen automatically on save

## Transition rules (proposed)
Allowed transitions:
- Owner actions:
  - `SUBMITTED -> READY_FOR_REVIEW`
  - `READY_FOR_REVIEW -> SUBMITTED` (pull back)
  - `CHANGES_REQUESTED -> READY_FOR_REVIEW` (resubmit)
- Reviewer actions:
  - `READY_FOR_REVIEW -> UNDER_REVIEW` (accept review)
  - `UNDER_REVIEW -> CHANGES_REQUESTED`
  - `UNDER_REVIEW -> APPROVED`
  - `UNDER_REVIEW -> REJECTED`

Disallowed transitions:
- Owner edits or owner transition to editable state from `UNDER_REVIEW`
- Reviewer direct acceptance from `SUBMITTED` to `UNDER_REVIEW`
- Any content mutation in `APPROVED` or `REJECTED` without reopening policy

## Enforcement requirements
- Enforce transition guards server-side at write time.
- Enforce state ownership in all mutation handlers.
- Use atomic from-state checks during status updates to avoid race conditions.
- Return user-friendly messages when a transition or save fails due to state change.

## Consequences
Positive:
- Prevents cross-party edit collisions without versioning.
- Makes ownership boundaries explicit in product and code.
- Improves operational clarity for both owners and reviewers.

Trade-offs:
- More explicit state transitions in UI and backend.
- Requires clear status messaging to avoid user confusion.

## Open items
- Confirm whether `CHANGES_REQUESTED -> SUBMITTED` should be permitted as an intermediate pullback.
- Define whether any non-material fields remain editable in `APPROVED`.
- Decide whether reviewer can return `UNDER_REVIEW -> READY_FOR_REVIEW` (no decision) or must choose one of outcome states.

## Implementation notes
- This ADR intentionally does not define a data versioning model.
- A follow-up design artifact will include a Mermaid state diagram aligned to this ADR.
