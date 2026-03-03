# ADR 0001: Stripe Checkout integration with donation intents and webhook finalization

- Status: Under Review
- Date: 2026-03-02
- Owners: Product + Engineering

## Context

We added a donation-intent MVP to capture donor intent before payment processing is wired.

Current needs:
- Integrate Stripe without breaking current intent flow.
- Ensure raised totals only include truly completed payments.
- Keep donor UX simple (hosted checkout) and implementation safe for pre-alpha.
- Preserve auditable state transitions from intent -> paid/finalized.

## Decision

Use Stripe **Hosted Checkout Sessions** as the payment entry point, with `donation_intents` as the system-of-record for donation lifecycle.

### Lifecycle

1. User submits donate form.
2. Server creates a `donation_intents` record with status `pending`.
3. Server creates a Stripe Checkout Session for that intent.
4. User is redirected to Stripe-hosted checkout.
5. Stripe webhook events (not browser redirect) drive finalization.
6. On confirmed payment, server marks intent `finalized` (or equivalent completed status).

### Source of truth for finalization

Finalization is driven by verified Stripe webhooks, not the success URL page.

Primary events to handle:
- `checkout.session.completed` (when `payment_status = paid`)
- `checkout.session.async_payment_succeeded`
- `checkout.session.async_payment_failed` (for failure handling)

Optional safety event:
- `payment_intent.succeeded` (secondary reconciliation signal)

### Correlation between Stripe and app data

Every Checkout Session must carry internal identifiers:
- `donation_intent_id`
- `need_id`
- optional `donor_user_id`

Use Checkout metadata and/or `client_reference_id` so webhook handlers can resolve internal records without ambiguity.

### Alternative considered: Payment Links

Payment Links were considered but not selected for this flow.

Why they are not the primary choice here:
- This app has per-need, per-intent dynamic checkout context (need, donor intent, chosen amount, anonymity/message).
- We need deterministic one-to-one correlation between internal `donation_intent` records and Stripe payment objects.
- Server-side business rules and lifecycle checks must run before redirecting to Stripe.

When Payment Links could still be useful:
- Static fundraising campaigns with fixed amounts and minimal per-transaction app state.

### Raised amount accounting

Only sum finalized/completed donations for progress:
- `WHERE need_id = :needID`
- `AND payment_status IN ('finalized')`

`pending`/failed/canceled states are excluded from raised totals.

## Implementation rules

1. **Do not finalize on success redirect page**
   - Success page is UX-only and may be visited without a completed payment.

2. **Verify webhook signatures**
   - Use Stripe webhook secret and reject invalid signatures.

3. **Idempotent webhook processing**
   - Stripe can retry delivery; handlers must be safe to run multiple times.
   - Persist processed Stripe event IDs (or equivalent idempotency guard).

4. **Server-authoritative amount**
   - Amount used for Stripe session is computed server-side from validated intent data.
   - Never trust client-modified amount post-submit.

5. **Auditable state model**
   - Persist Stripe IDs (`checkout_session_id`, `payment_intent_id`) on intent records.
   - Keep explicit status transitions (`pending` -> `finalized` / `failed` / `canceled`).

6. **Operational resilience**
   - Add a periodic reconciliation job to re-check non-finalized intents against Stripe in case of missed webhook delivery.

## Consequences

### Positive
- Safe payment finalization model aligned with Stripe best practices.
- Clear separation of intent vs completed funds.
- Reliable reporting and progress bars based on finalized payments only.
- Smooth migration path from MVP intent capture to real payments.

### Tradeoffs
- Requires webhook endpoint infrastructure and secret management.
- Requires idempotency/reconciliation logic for production robustness.
- Slightly more backend complexity than client-only confirmation.

## Non-goals (for this phase)

- Implementing custom Stripe Elements UI.
- Counting pending intents as raised.
- Trusting browser return URLs as payment truth.

## Next steps

1. Add Stripe config vars (`STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, optional publishable key).
2. Create Checkout Session endpoint from existing donate flow.
3. Add webhook endpoint + signature verification + idempotency storage.
4. Extend `donation_intents` with Stripe IDs and richer statuses.
5. Update need progress calculations to use finalized-only sums.
