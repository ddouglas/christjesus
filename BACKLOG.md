# Backlog (Low Priority / Later)

Use this list for ideas we want to do later but not right now.

## How to Add Items
- Keep each item short: action + outcome.
- Add tags: `impact` (`low|med|high`) and `effort` (`S|M|L`).
- Optional: add `revisit` note (milestone/date/dependency).

Example:
- [ ] Improve browse filters UX (`impact: low`, `effort: M`, `revisit: after donor MVP`)

---

## Later / Low Priority
- [ ] Add tests around auth navbar injection (`impact: med`, `effort: M`)
- [ ] Add end-to-end tests for donor onboarding flow (welcome → preferences → confirmation) (`impact: high`, `effort: M`)
- [ ] Add need-state guard across onboarding to block revisits/edits after submission (`impact: high`, `effort: M`)
- [ ] Add Profile "View Need" button + dedicated need review page (read-only first), then expand with secure reviewer/individual communications and supplemental document uploads (`impact: high`, `effort: L`, `revisit: after profile MVP hardening`)
- [ ] Add reusable breadcrumb component with proper breadcrumb builder for site-wide propagation (`impact: med`, `effort: M`, `revisit: after nav/layout consolidation`)
- [ ] Add webhook resilience pass: handle reconciliation signals, retries, and backfill job for missed events (`impact: high`, `effort: L`, `revisit: before production launch`)
- [ ] Improve post-donation trust UX with finalized-intent confirmation/receipt details (not redirect-only) (`impact: med`, `effort: M`, `revisit: after ledger MVP`)
- [ ] Implement working `Categories` navbar item + backed categories page route/render (data-driven cards/counts, desktop + mobile parity) (`impact: high`, `effort: M`, `revisit: align to docs/images/Categories - List.png + docs/images/Categories - HoverState.png + docs/images/Category - AssociatedNeeds.png + docs/images/Categories - BrowseCTA.png`)
- [ ] Enrich homepage with additional detail sections/content density while preserving current design system and hierarchy (`impact: med`, `effort: M`, `revisit: scope from docs/images/Homepage - NeedCategories.png + docs/images/Homepage - FeaturedNeed.png + docs/images/Homepage - FeaturedNeedsHomepage.png and follow-up product notes`)

### Next-Session Implementation Notes (Stripe Donations)

#### 1) Finalized-only raised/progress accounting
- Objective: funding progress must reflect only completed money movement, never pending intents.
- Current context:
	- `donation_intents.payment_status` is updated to `finalized` via Stripe webhook.
	- Donate flow and webhook finalization are working.
- Implementation plan:
	- Add repository query to sum `amount_cents` from `donation_intents` filtered by `need_id` + `payment_status = 'finalized'`.
	- Use this sum as the source for need progress/funding calculations shown in browse/detail/donate summaries.
	- Decide one strategy and keep it consistent:
		- read-time aggregation only, or
		- write-through projection to `needs.amount_raised_cents` on finalization.
- Acceptance checks:
	- A `pending` intent does not increase visible raised amount.
	- A `finalized` intent increases raised amount.
	- Replaying the same webhook event does not double-count.

#### 2) Donor donation ledger/history
- Objective: authenticated donor can view their donations and statuses.
- Scope (MVP):
	- New profile section/page listing donor’s donation intents by `donor_user_id`, newest first.
	- Columns/fields: created date, need reference (id/short descriptor), amount, status, anonymous flag.
	- Optional phase-2: status timeline (pending → finalized/failed/canceled) if we later persist transitions as events.
- Implementation plan:
	- Add repository method `DonationIntentsByDonorUserID`.
	- Add typed page data + template render from profile route group.
	- Keep UI read-only first (no edits/cancel actions in MVP).
- Acceptance checks:
	- Donor sees only their own intents.
	- Status reflects latest backend value.
	- Handles empty state cleanly.

#### 3) Webhook resilience (reconciliation + retries/backfill)
- Objective: eventual consistency even if webhooks are delayed, retried, or missed.
- Current context:
	- Signature verification and idempotency (`stripe_webhook_events`) already exist.
- Implementation plan:
	- Extend webhook handler for additional reconciliation signal(s), especially `payment_intent.succeeded`.
	- Add repository query for “stale non-finalized intents” (e.g., pending older than threshold).
	- Add CLI/cron-safe job to fetch Stripe object state for stale intents and reconcile local status.
	- Log reconciliation actions with intent id + stripe ids for auditability.
- Acceptance checks:
	- Duplicate webhook deliveries remain idempotent.
	- A missed `checkout.session.completed` can still be recovered by reconciliation.
	- Job is safe to run repeatedly.

#### 4) Post-donation trust UX (receipt/confirmation)
- Objective: confirmation experience tied to backend truth, not just redirect arrival.
- Current context:
	- Confirmation page exists and can be reached from success URL.
- Implementation plan:
	- Update confirmation page copy/state to reflect actual intent status (`pending/finalized/failed`).
	- If status is pending, show “processing” state and guidance.
	- If finalized, show receipt-style summary (amount, need, date, anonymous flag, intent id).
	- If failed/canceled, show recovery CTA (retry donation path).
- Acceptance checks:
	- Visiting success URL before webhook finalization does not falsely show success.
	- Finalized donation shows clear completion details.
	- Failed state is explicit and actionable.

## Done (Optional)
- Move completed items here with date if you want historical context.
- [x] Ensure raised amount/progress accounting is driven by finalized donation intents (and optionally project to need totals) (`impact: high`, `effort: M`, `revisit: after Stripe webhook MVP stabilization`) — 2026-03-03
- [x] Add donor donation ledger/history view with status timeline in profile (`impact: med`, `effort: M`, `revisit: after donation intent lifecycle hardening`) — 2026-03-03
- [x] Persist donor preferences to DB and prefill onboarding form (`impact: med`, `effort: M`, `revisit: after need flow polish`) — 2026-03-03
- [x] Add donor onboarding completion confirmation page (`impact: low`, `effort: S`) — 2026-03-03
