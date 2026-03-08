# Admin Interface Plan (Working Document)

- Status: Draft
- Date: 2026-03-08
- Owners: Product + Engineering
- Branch: `feat/admin`

## Purpose

Define an incremental plan for an admin interface we can ship safely, and track architecture/product decisions in one place.

## Problem Statement

The app currently supports donor/need user flows, but there is no first-class internal admin surface for moderation and operations.

We need an admin interface to:
- Review and manage submitted needs.
- Verify uploaded documents and track verification outcomes.
- Manage user access controls (admin group membership).
- Resolve donation or profile support issues.
- Perform core operational tasks without direct DB access.

## Goals (Phase 1)

- Add authenticated admin-only pages in the existing Go SSR app.
- Enable need moderation lifecycle actions: queue -> review -> approve/reject/request changes.
- Show key context on each need: profile, location, categories, story, documents, timeline.
- Record admin actions as explicit events for auditability.
- Allow soft-delete/restore from admin UI (no hard delete in UI).
- Keep initial UX simple and reliable over broad feature scope.

## Non-Goals (Phase 1)

- Full back-office analytics suite.
- Bulk import/export tools.
- Granular per-field edit history UI.
- Multi-tenant admin segmentation.

## Principles

- Reuse existing stack and conventions (Flow routes, typed page data, `renderTemplate`, repositories).
- Prefer server-rendered pages + PRG pattern for actions.
- Enforce authorization server-side on every admin route.
- Add auditable state transitions (who, what, when, why).
- Ship thin vertical slices; avoid a giant admin "big bang".

## Proposed Access Model

### Authentication
- Reuse existing Cognito login/session model (`AttachAuthContext`).

### Authorization
- Add explicit admin role checks (`RequireAdmin`) on admin route group.
- Authorization source is Cognito Group membership.
- Admin users are added/removed from the Cognito admin group via admin UI.
- Maintain one IaC-managed super-user with durable break-glass access.
- Super-user credential storage in Secrets Manager is required operationally but out of scope for this branch.

## Proposed Information Architecture

- `/admin` (dashboard)
- `/admin/needs` (queue/list)
- `/admin/needs/:id` (detail/review)
- `/admin/users/:id` (basic profile/support context)
- `/admin/audit` (event list; Phase 1.5 if time)

## Need Moderation Workflow (Proposed)

Need state progression for admin review:
1. `SUBMITTED` (user completed onboarding)
2. `UNDER_REVIEW` (admin has started review)
3. `APPROVED` or `REJECTED` or `CHANGES_REQUESTED`

Phase 1 moderation constraints:
- No admin edits to posts/needs.
- Admins moderate only (approve/reject/request changes) and do not edit need fields directly.

User-management scope (separate from need moderation):
- Admins can edit user access controls (for example, admin group membership).
- This does not grant permission to edit need content.

Action events to persist:
- `review_started`
- `review_note_added`
- `changes_requested`
- `review_approved`
- `review_rejected`
- `document_verified` / `document_rejected`

## Data/Schema Additions (Draft)

1. `need_progress_events` (or new `admin_events` table)
- Ensure event records include actor user ID and event metadata.
- Confirm current model can represent moderation actions; extend if needed.

2. Optional moderation fields on `needs`
- `reviewed_by_user_id`
- `reviewed_at`
- `review_decision_reason` (nullable text)
- `changes_requested_at`
- `changes_requested_by_user_id`
- `changes_request_reason`

3. Soft-delete fields (or equivalent) on admin-managed entities
- Example on `needs`: `deleted_at`, `deleted_by_user_id`, `delete_reason`.
- Keep rows/data recoverable through restore actions.

## Backend Changes (Slice Plan)

### Slice 1: Admin auth shell
- Add admin middleware (`RequireAdmin`).
- Add basic admin home route/template.
- Add minimal nav entry visible only to admins.

### Slice 2: Needs queue
- Repository query for submitted needs with filters.
- SSR list page with status, created date, location, categories.

### Slice 3: Need review page
- Render complete read-only need packet (story/docs/progress).
- Add approve/reject/request-changes POST actions with PRG redirects.
- Persist moderation event records.
- Render moderation audit timeline in-page.

### Slice 4: Document verification actions
- Mark docs verified/rejected with notes.
- Reflect document verification state on review page.

### Slice 5: Admin access management + soft-delete controls
- Add admin UI flow to add/remove users from Cognito admin group.
- Add soft-delete/restore actions for needs with required reason capture.
- Ensure all delete/restore actions write explicit audit events.

## Frontend/UI Notes

- Keep visuals aligned with current design system and Tailwind token usage.
- Prioritize scanability: status chips, clear timestamps, concise event timeline.
- No bespoke frontend framework; stay in existing server templates.

## Testing Strategy

- Unit tests for admin middleware authorization logic.
- Handler tests for admin route guards and moderation state transitions.
- Repository tests for admin queue/filter queries.
- Template render smoke tests for admin pages.

## Risks and Mitigations

- Risk: Authorization gaps on new routes.
  - Mitigation: central admin route group + explicit middleware tests.

- Risk: Unclear moderation audit trail.
  - Mitigation: event-first write model for every admin action.

- Risk: Scope creep in first admin release.
  - Mitigation: strict slice-based milestone gating.

## Milestones

- M1: Admin auth shell + dashboard route.
- M2: Needs queue list.
- M3: Need review page with approve/reject/request changes + audit timeline.
- M4: Document verification + review notes.
- M5: Admin group management + soft-delete/restore controls.

## Decision Log

| ID | Date | Topic | Decision | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| ADM-001 | 2026-03-08 | Admin authorization source | Use Cognito Group membership for `RequireAdmin` checks | Accepted | Admin group membership managed via admin UI; retain IaC-managed super-user |
| ADM-002 | 2026-03-08 | Admin UI stack | Use existing Go SSR + templates | Accepted | Avoid introducing separate admin SPA |
| ADM-003 | 2026-03-08 | Moderation auditability | Persist explicit admin action events | Accepted | Reuse `need_progress_events` if suitable |
| ADM-004 | 2026-03-08 | Post-submission content edits | Admins cannot edit posts/needs | Accepted | Moderation only: approve/reject/request changes |
| ADM-005 | 2026-03-08 | Request changes state | Include `CHANGES_REQUESTED` in Phase 1 moderation | Accepted | Admins can request changes without editing need fields directly |
| ADM-006 | 2026-03-08 | Admin role management path | Managed via admin UI (Cognito group membership) | Accepted | Super-user bootstrapped and maintained by IaC |
| ADM-007 | 2026-03-08 | Delete behavior | Soft-delete/restore in UI; no hard delete in UI | Accepted | Hard delete remains manual ops path only |
| ADM-008 | 2026-03-08 | Edit permissions boundary | Admins may edit users/access controls, but may not edit submitted need content | Accepted | Keeps moderation auditable while preserving submitter-authored need integrity |
| ADM-009 | 2026-03-08 | Audit visibility in moderation | Need review page must display moderation timeline | Accepted | Review decisions require full moderation context |

## Resolved Questions (2026-03-08)

1. Admin edits to submitted content:
 - Decision: Not allowed. Admins moderate but do not edit need content.

1a. Admin edits to users/access controls:
- Decision: Allowed in Phase 1 (starting with Cognito group membership management).

2. Request changes state:
- Decision: Included in Phase 1 (`CHANGES_REQUESTED`).

3. Admin role management:
- Decision: Admin UI manages Cognito admin group membership.

4. Soft-delete/restore in UI:
- Decision: Included in Phase 1+ scope; UI must not hard delete data.

## Next Discussion Agenda

1. Confirm exact Cognito group name(s), claim shape, and middleware mapping.
2. Confirm soft-delete schema shape and restore UX rules.
3. Lock moderation event schema (event type + payload) and begin coding.
