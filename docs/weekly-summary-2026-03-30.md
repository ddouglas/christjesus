# Weekly Build Summary — Week of March 23–30, 2026

~25 PRs merged across email infrastructure, geospatial search, USPS validation, donor experience, admin tooling, E2E testing, and a routing refactor.

---

## Transactional Email (Resend)

- **#88** — Full email infrastructure: `Sender` interface, `ResendSender` implementation, 5 new DB tables (`email_messages`, `email_events`, `email_suppressions`, `donation_intent_emails`, `user_emails`), store layer, and Resend webhook handler with Svix signature verification + suppression auto-population
- **#91** — Wired the email sender into the server; triggers donation receipt emails on Stripe `checkout.session.completed` / `payment_intent.succeeded`
- **#92** — Local dev tooling: `just ngrok-resend` + `just resend-local` for forwarding Resend webhooks locally

---

## Geospatial Search

- **#51** — `zip_centroids` table + `import-zips` CLI command (loads ~33k US Census ZCTA rows)
- **#54** — Browse page ZIP + radius filter using PostGIS `ST_DWithin`; replaces city dropdown; distance shown on need cards
- **#56** — Fixed geo browse query (count query column error), geography type schema reference, `derefFloat` template helper for `*float64` display

---

## USPS Address Validation

- **#50** — Full USPS API client with OAuth2 token caching; validates and standardizes addresses on need creation/edit; gracefully skips on USPS outages

---

## Donor Experience

- **#49** — Large omnibus: donor onboarding skip flow, smart preset amounts, "Recommended for you" on home, impact stats on donor profile
- **#58** — Dedicated `/profile/preferences` page for donors to view/edit saved preferences
- **#60** — Browse page auto-applies saved preferences as default filters, with "Using your preferences / Disable" toggle
- **#80** — Smart donation presets: filters out amounts exceeding remaining balance; adds a gold "Fund the remaining $X" CTA
- **#82** — Saved/bookmarked needs: bookmark button on need detail, "Saved Needs" section on profile with remove action
- **#84** — "Skip for now" link on donor preferences onboarding step
- **#85** — "Needs like this" section on donation confirmation (up to 3 same-category needs)
- **#87** — "Recommended for you" on home page using saved category + location preferences

---

## Onboarding / UX

- **#81** — Added and fixed progress steppers across all onboarding flows (donor 3-step, need 7-step)
- **#77** — Fixed nav z-index so dropdown isn't hidden behind hero sections

---

## Profile & Auth Fixes

- **#61** — Profile edit controls: display name, email update, password reset (Auth0 ticket flow)
- **#62** — "Submit a Need" CTA added to empty-state and section header on recipient profile
- **#73** — Fixed `DisplayName` not being persisted in `authUserState` cookie on login (caused name reversion after logout)
- **#75** — Fixed Auth0 post-login Action to inject `display_name` claim using the correct fallback chain

---

## Admin

- **#44** — Admin user management: `/admin/users` list (search, type filter, pagination) + user detail pages showing need/donation history
- **#79** — `urgency` column on needs table; admins set urgency at approval time and can update it inline; browse filter/sort now operates on stored value instead of computed funding %

---

## Refactors & Infrastructure

- **#71** — Replaced `map[string]string` route params with a `RouteOption` functional options pattern across 164 Go + 71 template call sites
- **#74** — Fixed fatal template parse error from unregistered `dict` calls (replaced with `param`)
- **#70** — Extracted `buildNeedSummaries` / `buildDonationSummaries` from `handleGetProfile`
- **#43** — Renamed user type `"need"` → `"recipient"` throughout
- **#55** — Playwright E2E test infrastructure: Auth0 test users via Terraform, login helpers, `e2e-reset` CLI, full recipient onboarding flow test
