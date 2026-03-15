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
- Reuse existing Auth0 login/session model (`AttachAuthContext`).

### Authorization
- Add explicit admin role checks (`RequireAdmin`) on admin route group.
- Authorization source is Auth0 role claim membership.
- Admin users are added/removed from the Auth0 `admin` role via admin tooling.
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
- Decision: reuse `need_progress_events` for moderation timeline events.
- Extend current model as needed so moderation actions can be represented without adding a second events table.
- Constraint: do not store moderation detail content (approval/reject reasons, note bodies) directly in `need_progress_events`.
- Store detail content in a dedicated moderation detail table and keep only references/IDs in timeline events.

1a. `need_moderation_actions` (new table)
- Stores moderation detail payloads (reason text, note text, optional document context, actor, timestamps).
- `need_progress_events` stores an action reference (`moderation_action_id`) so timeline can resolve details when needed.

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
- Add admin UI flow to add/remove users from Auth0 admin role.
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

## Implementation Roadmap

### Issue Mapping

- `#11` M1 Admin auth shell: completed.
- `#12` Moderation event schema and persistence: next in progress target.
- `#13` M2 Needs queue page (`/admin/needs`).
- `#14` M3 Need review page + moderation actions.
- `#15` M4 Document verification actions.
- `#16` M5 Access management + soft-delete/restore controls.

### Execution Order

1. Implement `#12` first to establish canonical event writes/reads.
2. Implement `#13` queue page on top of current admin shell.
3. Implement `#14` review detail page + approve/reject/request-changes.
4. Implement `#15` document verification actions and timeline events.
5. Implement `#16` access management and soft-delete/restore.

### File-Level Plan

`#12` Moderation events backbone:
- `migrations/need_progress_events.pg.hcl`: extend schema for actor and moderation action reference fields.
- `migrations/need_moderation_actions.pg.hcl`: add dedicated table for moderation detail content.
- `pkg/types/need_progress_event.go` (or existing types file): add typed moderation event model/constants.
- `pkg/types/need_moderation_action.go` (or existing types file): add typed moderation action model.
- `internal/store/need_progress.go`: add repository writes and ordered timeline reads.
- `internal/store/need_moderation_action.go`: add create/get methods for moderation detail records.
- `internal/server/`: introduce a small service helper for writing moderation events consistently.

`#13` Needs queue page:
- `internal/server/routes.go`: add `admin.needs` route constant/pattern.
- `internal/server/server.go`: wire admin queue GET route under `RequireAdmin`.
- `internal/server/admin_needs.go`: queue handler.
- `pkg/types/page_data.go`: typed queue page data structs.
- `internal/server/templates/pages/admin-needs.html`: queue template.
- `internal/store/need.go` (or relevant repo): moderation queue query.

`#14` Need review + moderation actions:
- `internal/server/routes.go`: add detail/action routes for `/admin/needs/:id`.
- `internal/server/admin_need_review.go`: GET detail + POST moderation action handlers.
- `pkg/types/page_data.go`: review page data + timeline item types.
- `internal/server/templates/pages/admin-need-review.html`: detail and action UI.
- `internal/store/need.go`: status transition helpers with safeguards.
- `internal/store/need_progress.go`: timeline read + moderation event writes.

`#15` Document verification:
- `internal/server/admin_need_review.go` (or split file): verify/reject document POST handlers.
- `internal/store/document.go`: verification status update methods.
- `internal/store/need_progress.go`: write `document_verified`/`document_rejected` events.
- `internal/server/templates/pages/admin-need-review.html`: document action controls.

`#16` Access management + soft-delete:
- `internal/server/routes.go`: admin user/access routes + soft-delete routes.
- `internal/server/admin_users.go`: Auth0 role membership handlers.
- `internal/server/admin_needs_delete.go`: soft-delete/restore handlers.
- `internal/store/need.go`: soft-delete/restore persistence and queries.
- `migrations/needs.pg.hcl`: add soft-delete columns if approved unchanged.

### Sprint 1 Definition (Start Now)

- Goal: complete `#12` and begin `#13` in same branch.
- Done criteria for Sprint 1:
  - Moderation event schema is implemented on `need_progress_events`.
  - Repository read/write tests for moderation events pass.
  - `/admin/needs` route and minimal list rendering works behind `RequireAdmin`.
  - `go test ./...` passes.

## Decision Log

| ID | Date | Topic | Decision | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| ADM-001 | 2026-03-08 | Admin authorization source | Use Auth0 role claim membership for `RequireAdmin` checks | Accepted | Admin role membership managed via admin tooling; retain IaC-managed super-user |
| ADM-002 | 2026-03-08 | Admin UI stack | Use existing Go SSR + templates | Accepted | Avoid introducing separate admin SPA |
| ADM-003 | 2026-03-08 | Moderation auditability | Persist explicit admin action events | Accepted | Reuse `need_progress_events` if suitable |
| ADM-004 | 2026-03-08 | Post-submission content edits | Admins cannot edit posts/needs | Accepted | Moderation only: approve/reject/request changes |
| ADM-005 | 2026-03-08 | Request changes state | Include `CHANGES_REQUESTED` in Phase 1 moderation | Accepted | Admins can request changes without editing need fields directly |
| ADM-006 | 2026-03-08 | Admin role management path | Managed via admin UI/tooling (Auth0 role membership) | Accepted | Super-user bootstrapped and maintained by IaC |
| ADM-007 | 2026-03-08 | Delete behavior | Soft-delete/restore in UI; no hard delete in UI | Accepted | Hard delete remains manual ops path only |
| ADM-008 | 2026-03-08 | Edit permissions boundary | Admins may edit users/access controls, but may not edit submitted need content | Accepted | Keeps moderation auditable while preserving submitter-authored need integrity |
| ADM-009 | 2026-03-08 | Audit visibility in moderation | Need review page must display moderation timeline | Accepted | Review decisions require full moderation context |
| ADM-010 | 2026-03-08 | Event storage model | Reuse `need_progress_events` for moderation events | Accepted | Avoids joining across multiple event tables |
| ADM-011 | 2026-03-08 | Moderation detail storage boundary | Keep detailed moderation content in dedicated table and reference it from events | Accepted | Prevents storing rich approval/rejection content directly in timeline events |

## Resolved Questions (2026-03-08)

1. Admin edits to submitted content:
 - Decision: Not allowed. Admins moderate but do not edit need content.

1a. Admin edits to users/access controls:
- Decision: Allowed in Phase 1 (starting with Auth0 role membership management).

2. Request changes state:
- Decision: Included in Phase 1 (`CHANGES_REQUESTED`).

3. Admin role management:
- Decision: Admin UI manages Auth0 admin role membership.

4. Soft-delete/restore in UI:
- Decision: Included in Phase 1+ scope; UI must not hard delete data.

## Next Discussion Agenda

1. Confirm exact Auth0 role claim key/value shape and middleware mapping.
2. Confirm soft-delete schema shape and restore UX rules.
3. Define moderation event payload fields on `need_progress_events` and begin coding.
