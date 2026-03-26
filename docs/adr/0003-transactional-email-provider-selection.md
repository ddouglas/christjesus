# ADR 0003: Transactional email provider selection

- Status: Accepted
- Date: 2026-03-25
- Owners: Engineering

## Context

The application needs transactional email for account verification, password resets, operational notifications, and eventually nonprofit-related emails such as donation acknowledgments and receipts. The stack is Golang on Fly.io with Supabase Postgres and Tigris object storage — intentionally non-AWS.

We need a provider that fits this architecture: simple API key authentication, direct HTTPS API or SMTP integration, first-class webhook delivery for bounce/complaint/delivery events handled by the Fly.io-hosted app, and event persistence into Supabase Postgres.

## Options evaluated

### AWS SES

Cheapest at scale (~$0.10/1,000 emails). However, it is a poor architectural fit because it pulls the project into AWS-native event plumbing (SNS topics for bounces/complaints, EventBridge for routing, IAM for credentials). This contradicts the project's deliberate non-AWS posture and adds operational complexity disproportionate to the email volume expected in early phases.

### Resend

Modern developer-first transactional email service. Clean REST API with official and community Go support. Direct webhook delivery for bounce, complaint, and delivery events — no intermediate message bus required. Simple API key auth model. Free tier of 3,000 emails/month covers early development. Paid plans start at $20/month for 50,000 emails. Strong alignment with the project's emphasis on developer experience and operational simplicity.

### MailerSend

Solid transactional email with good API design and webhook support. Offers nonprofit discounting, which is relevant since the project is expected to eventually become a 501(c)(3). Free tier of 3,000 emails/month. Go SDK available. Slightly less polished developer experience compared to Resend but functionally comparable. The nonprofit pricing could become a differentiator at scale.

### Postmark

Industry-leading deliverability reputation, especially for transactional email. Strong webhook support. However, higher per-message cost ($1.25/1,000 emails) and weaker Go ergonomics compared to Resend. Better suited for applications where email deliverability is the single most critical operational concern.

### SMTP2GO and Brevo

Both are viable general-purpose email providers with SMTP and API support. Neither stands out for this stack — less modern developer experience, less aligned with the API-first integration model preferred here. Brevo's strength is in marketing email, which is not a current need.

## Decision

Standardize on **Resend** for the initial transactional email implementation.

The integration will include a **provider-agnostic email interface** in Go so the underlying provider can be swapped later without modifying calling code. This preserves flexibility to move to MailerSend (for nonprofit pricing), Postmark (for deliverability), or another provider as needs evolve.

### Integration architecture

1. **Provider-agnostic interface** — A Go `EmailSender` interface in the application that abstracts send operations, template rendering, and provider-specific details behind a common contract.

2. **Webhook endpoint** — An authenticated endpoint on Fly.io (e.g., `POST /webhooks/resend`) that receives bounce, complaint, and delivery events directly from the provider. Webhook signature verification required, following the same pattern as the existing Stripe webhook endpoint.

3. **Database tables** — Postgres tables in the `christjesus` schema for:
   - `email_messages` — record of every sent email (recipient, type, provider message ID, status, timestamps)
   - `email_events` — webhook event log (event type, provider event ID, message reference, payload, timestamp)
   - `email_suppressions` — suppression list derived from bounce/complaint events (email address, reason, source event, timestamps)

4. **Configuration** — API key via environment variable (`RESEND_API_KEY`), set as a Fly.io secret. Webhook signing secret via `RESEND_WEBHOOK_SECRET`.

### Why not SES

Even though the team has strong AWS experience and SES is the cheapest option at scale, it is the wrong choice here. The project has deliberately avoided AWS services in favor of a simpler, more cohesive stack. Introducing SES would require SNS for bounce/complaint routing, IAM credentials instead of simple API keys, and AWS-specific operational patterns that conflict with the Fly.io + Supabase architecture. The cost savings do not justify the architectural divergence at the expected email volumes.

## Consequences

### Positive

- Stack coherence — email integration follows the same patterns as Stripe (API key + webhooks + Postgres persistence) with no new infrastructure dependencies.
- Operational simplicity — no message buses, IAM policies, or cloud-provider event routing to manage.
- Future flexibility — provider-agnostic interface allows swapping to MailerSend, Postmark, or others without application-layer changes.
- Fast initial implementation — Resend's API is minimal and well-documented; webhook model is straightforward.

### Tradeoffs

- Resend is a younger company with a smaller track record than SES or Postmark. Mitigated by the abstraction layer.
- Higher per-message cost than SES at scale. Acceptable given expected volumes and the value of architectural simplicity.
- If the project obtains 501(c)(3) status and email volume grows significantly, MailerSend's nonprofit pricing may become more attractive — the abstraction layer makes this switch feasible.

## Non-goals (for this phase)

- Marketing or bulk email campaigns.
- Email template management in the provider's UI (templates will be rendered server-side in Go).
- Multi-provider failover or redundancy.
- Integration with Auth0's email provider settings (Auth0 manages its own verification/reset emails separately).
