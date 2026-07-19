# CLAUDE.md — christjesus

## Project Overview

A Go web application for managing charitable needs ("Body of Christ" — bodyofchrist.app, formerly christjesus.app). Server-side rendered (SSR) with PostgreSQL, Auth0 authentication, Tigris object storage, Stripe payments, and Render hosting (deployed via GHCR).

- **Module:** `christjesus`
- **Go version:** 1.26 (Dockerfile build image; go.mod may lag — check before assuming)
- **Task runner:** `just` (justfile), which auto-loads `.env`

---

## Running Locally

Local dev secrets (DB, Auth0, Stripe, Tigris, etc.) come from the SOPS-encrypted `configs/local/app.enc.yaml` — not a hand-filled `.env`. See [Secrets Management](#secrets-management--sops--age) to get set up with an age key first.

```bash
# 1. Apply schema migrations (against the remote dev DB in configs/local/app.enc.yaml)
sops exec-env configs/local/app.enc.yaml 'just migrate'

# 2. (Optional) Seed categories and fake data
sops exec-env configs/local/app.enc.yaml 'just seed'

# 3. Start with hot reload (recommended)
just dev
```

The app runs on `http://localhost:8080`. `just dev` uses `air` for hot reload (watches `.go`, `.html`, `.css`, `.js`, `.tpl`) and already wraps itself in `sops exec-env` — no manual wrapping needed for that one recipe.

**Note:** `configs/local/app.enc.yaml`'s `DATABASE_URL` currently points at the **remote Supabase dev database**, shared across contributors — not a local Postgres instance. `docker-compose.yaml`'s `db` service exists for local/isolated Postgres use (e.g. testing Atlas migrations against a throwaway DB) but the app doesn't use it by default.

Any other recipe that touches the DB or external services and isn't already SOPS-wrapped (`just serve`, `just build`+run, `just test` against a real DB, etc.) needs the same treatment: `sops exec-env configs/local/app.enc.yaml '<command>'`.

**Run in Docker instead of natively:**
```bash
just docker-up       # sops exec-env + docker compose up --build app, same secrets, containerized
```

Alternative startup commands:
```bash
just serve          # go run ./cmd/christjesus serve (needs sops exec-env wrapping — see above)
just build          # compiles to ./.bin/christjesus
```

---

## Build & Development Commands

| Command | Description |
|---|---|
| `just dev` | Run with hot reload via `air` |
| `just serve` | Run without hot reload |
| `just build` | Compile binary to `./.bin/christjesus` |
| `just test` | Run all tests (`go test -count=1 ./...`) |
| `just fmt` | Format code (`go fmt ./...`) |
| `just seed` | Seed DB with categories and fake data |
| `just deps` | Tidy Go modules (`go mod tidy`) |
| `just migrate` | Apply Atlas schema to `DATABASE_URL` |
| `just migrate-plan` | Dry-run schema migration |
| `just import-zips` | Import ZIP centroid data (geo/browse features) |
| `just webhooks` | Spin up ngrok and forward Stripe/Resend webhooks to localhost |
| `just e2e` / `e2e-headed` / `e2e-ui` | Run Playwright e2e suite (resets DB first) |
| `just e2e-reset` | Reset DB state for e2e |
| `just tf-init` | `terraform init` |
| `just tf-plan/apply/console/state-list [workspace]` | Terraform ops against SOPS-decrypted tfvars (default workspace `development`) — see [Secrets Management](#secrets-management--sops--age) |
| `just docker-up` | Build and run the app in Docker via `docker compose`, secrets injected via `sops exec-env` |

---

## Environment Variables

`just` loads `.env` automatically (`set dotenv-load` in the justfile), but `.env`/`.env.example` today only holds `SOPS_AGE_KEY_FILE` — actual app config comes from SOPS-encrypted files (see [Secrets Management](#secrets-management--sops--age)), not a hand-filled `.env`. The table below documents what the running process expects in its environment, regardless of how it gets there.

**Required:**
| Variable | Description |
|---|---|
| `DATABASE_URL` | PostgreSQL connection string |
| `COOKIE_HASH_KEY` | Base64 HMAC key for cookie signing (32 or 64 bytes) |
| `COOKIE_BLOCK_KEY` | Base64 AES key for cookie encryption (16, 24, or 32 bytes) |
| `AUTH0_DOMAIN` | Auth0 tenant domain (e.g. `yourtenant.us.auth0.com`) |
| `AUTH0_CLIENT_ID` | Auth0 application client ID |
| `AUTH0_CLIENT_SECRET` | Auth0 application client secret |
| `AUTH0_MGMT_CLIENT_ID` | Auth0 M2M client ID for Management API (profile updates) |
| `AUTH0_MGMT_CLIENT_SECRET` | Auth0 M2M client secret for Management API |
| `S3_BUCKET_NAME` | Tigris bucket name for document uploads |
| `USPS_CONSUMER_KEY` | USPS API OAuth2 client ID for address validation |
| `USPS_CONSUMER_SECRET` | USPS API OAuth2 client secret for address validation |

**Optional with defaults:**
| Variable | Default | Description |
|---|---|---|
| `ENVIRONMENT` | `development` | Runtime environment label |
| `SERVER_PORT` | `8080` | HTTP listen port |
| `APP_BASE_URL` | `http://localhost:8080` | Base URL for absolute links |
| `AUTH0_CALLBACK_URL` | `http://localhost:8080/auth/callback` | OAuth callback URL |
| `AUTH0_LOGOUT_URL` | `http://localhost:8080/` | Post-logout redirect |
| `AUTH_ADMIN_CLAIM` | `https://christjesus.app/claims/roles` | JWT claim containing roles |
| `AUTH_ADMIN_VALUE` | `admin` | Role value granting admin access |
| `OBJECT_STORE_ENDPOINT` | `https://t3.storage.dev` | Tigris S3-compatible endpoint |

**Optional (no default):**
| Variable | Description |
|---|---|
| `STRIPE_SECRET_KEY` | Stripe secret key (`sk_...`) |
| `STRIPE_PUBLISHABLE_KEY` | Stripe publishable key (`pk_...`) |
| `STRIPE_WEBHOOK_SECRET` | Stripe webhook signing secret |
| `TIGRIS_ACCESS_KEY` | Tigris S3 access key |
| `TIGRIS_SECRET_KEY` | Tigris S3 secret key |

---

## Database — Supabase / PostgreSQL

- **Production:** Supabase-hosted PostgreSQL (connection string in `DATABASE_URL`)
- **Local dev:** By default, `configs/local/app.enc.yaml` points `DATABASE_URL` at a **shared remote Supabase dev database** — not a local instance. `docker-compose.yaml` also runs `postgres:18-alpine` on `localhost:5432` (user/pass/db all `christjesus`) if you want an isolated local DB instead; override `DATABASE_URL` to use it.
- **Schema tool:** [Atlas](https://atlasgo.io/) with HCL schema files in `migrations/`
- **Schema namespace:** All tables live in the `christjesus` schema (not `public`); `search_path` is set at connection time
- **Primary keys:** NanoID text strings, not UUIDs or auto-increment integers
- **Connection pool:** `pgx/v5` via `internal/db/postgres.go`

Schema files are per-table HCL files in `migrations/`. To update the schema, edit the relevant `.pg.hcl` file and run `just migrate`.

---

## Auth Platform — Auth0

Auth0 handles identity using the **Authorization Code (OIDC)** flow.

**Flow:**
1. User visits `/auth/login` or `/auth/register` → redirected to Auth0
2. Auth0 redirects to `/auth/callback` with an authorization code
3. Server exchanges code for tokens using `client_secret_post`
4. ID token JWT is validated against Auth0's JWKS endpoint
5. User identity is stored in encrypted `gorilla/securecookie` cookies

**Admin roles:** An Auth0 post-login Action injects roles into the ID token as the custom claim `https://christjesus.app/claims/roles`. A user with the `admin` role in Auth0 gets admin access in the app.

**Cookies used:**
- `cja_access_token` — encrypted Auth0 ID token
- `cja_auth_user_state` — encrypted user metadata
- `cja_auth_state` — CSRF state during OAuth flow
- `cja_auth_nonce` — nonce for ID token replay protection
- `cja_csrf` — gorilla/csrf CSRF token

Auth0 infrastructure (tenant, client, roles, actions) is managed via Terraform in `terraform/`.

**Auth0 CLI** — useful for local admin tasks (bulk user deletion, inspecting users, etc.):

```bash
brew tap auth0/auth0-cli
brew install auth0
auth0 login
```

Bulk delete all users in a connection:
```bash
auth0 users search --query "*" --json \
  | jq -r '.[].user_id' \
  | xargs -I {} auth0 users delete {}
```

---

## Container Platform — Render

**Deployment moved from Fly.io to Render.** CI builds and pushes the image to GHCR, then calls a Render deploy hook — see `.github/workflows/build-and-deploy.yml`. Render infra (project, environment, service, custom domain `development.bodyofchrist.app`) is provisioned by `terraform/render.tf`. `fly.toml` was removed as dead config (Fly is no longer part of the deploy path).

The Dockerfile is a two-stage build:
1. `golang:1.26-alpine` — builds binary with `CGO_ENABLED=0 GOOS=linux`
2. `alpine:3.21` — minimal runtime image, runs `christjesus serve`

**No test/lint gate runs in CI** — `.github/workflows/build-and-deploy.yml` builds and deploys on push to `main`/tags without running `just test` first.

---

## Secrets Management — SOPS + age

Secrets live encrypted-at-rest in git as SOPS files, decrypted locally or by Terraform at apply time. **Nothing here should ever be committed in plaintext.**

- **Encryption tool:** [SOPS](https://github.com/getsops/sops) with an [age](https://github.com/FiloSottile/age) recipient (no PGP/KMS).
- **Recipient config:** `.sops.yaml` (repo root) and `terraform/.sops.yaml` — **keep these two files in sync**, both list the same age public key(s).
- **Encrypted files:**
  | File | Consumed by |
  |---|---|
  | `configs/development/terraform.enc.yaml` | Terraform, via `data.sops_file` (`terraform/sops.tf`, `carlpett/sops` provider) |
  | `configs/development/app.enc.yaml` | Terraform, via `data.sops_file` |
  | `configs/development/terraform.tfvars.enc.json` | `just tf-plan/apply/console/state-list` (decrypted to a tempfile and passed as `-var-file`) |
  | `configs/local/app.enc.yaml` | `just dev` (`sops exec-env` injects decrypted vars into the `air` process) |
- **Private key location:** expected at `~/.age/key.txt` (hardcoded in the `dev` justfile recipe) or SOPS' standard lookup (`SOPS_AGE_KEY_FILE` env var, or `~/.config/sops/age/keys.txt`).
- **Onboarding a new holder of secrets:**
  1. They run `age-keygen -o ~/.age/key.txt` and send you the printed public key (`age1...`) — never the private key.
  2. Add their public key to **both** `.sops.yaml` and `terraform/.sops.yaml`.
  3. Rekey every encrypted file so it's readable by the new recipient: `sops updatekeys -y <file>` for each file listed above.
  4. Commit the `.sops.yaml` changes and the rekeyed files together.
- **Rotating out** an old holder: remove their public key from both `.sops.yaml` files, rekey all files the same way, and rotate any underlying credentials they had access to (Auth0, Stripe, Tigris, DB) since they held the means to decrypt them.

---

## Project Structure

```
cmd/christjesus/      # CLI entry point (serve, seed, reconcile)
internal/
  db/                 # pgxpool connection setup
  server/             # HTTP handlers, middleware, templates, routes
  seed/               # Dev data seeding logic
  store/              # Repository layer (one file per entity)
  utils/              # Helpers (nano IDs, pointers, struct utilities)
pkg/types/            # Shared types: Config, User, Need, etc.
migrations/           # Atlas HCL schema files (one per table)
terraform/            # Auth0, Tigris, Cloudflare, Render infrastructure (Terraform)
configs/              # SOPS-encrypted per-workspace config (terraform + app secrets)
docs/                 # ADRs, design documents, QA plans
```

---

## Key Conventions

- **Router:** `github.com/alexedwards/flow`
- **SQL:** `github.com/Masterminds/squirrel` (query builder) + `github.com/georgysavva/scany/v2` (row scanning)
- **Config:** `github.com/kelseyhightower/envconfig` — reads from environment into `pkg/types/Config`
- **Logging:** `github.com/sirupsen/logrus` (structured JSON)
- **CSRF:** All state-mutating routes use `gorilla/csrf` except `/webhooks/stripe` (uses Stripe signature verification)
- **Templates:** Embedded via `//go:embed` and served SSR — no frontend framework
- **CSS:** Tailwind via CDN with custom design tokens in `internal/server/static/tokens.css`. Colors defined as CSS variables **cannot** be used as bare Tailwind utility classes (e.g. `bg-midnight` won't work). Use the bracket syntax instead: `bg-[#0D1B2A]` or `text-[color:var(--cj-secondary)]`. The existing `[color:var(--cj-*)]` pattern is the established convention.
- **Design tokens:** Palette is gold (`#C9A84C`) + midnight (`#0D1B2A`) + cream (`#FAF7F0`) + warm ink text. Fonts are Jost (body) + Cormorant Garamond (headings/display).
- **Tests:** Standard library `testing` only — no testify or gomock
- **Formatting:** `go fmt` (no golangci-lint configured)

---

## Infrastructure

- **Object storage:** Tigris (S3-compatible) — not Supabase Storage. `internal/storage/supabase.go` is a legacy file and not active.
- **Payments:** Stripe (donation intents + webhooks)
- **DNS:** Cloudflare, zone `bodyofchrist.app` (`terraform/cloudflare.tf`) — MX/TXT records point mail to Zoho; `development.bodyofchrist.app` CNAMEs to the Render service.
- **Hosting:** Render (`terraform/render.tf`) — see [Container Platform](#container-platform--render-current--flyio-legacy-likely-stale).
- **IaC:** Terraform Cloud, org `christjesus`, workspace `christjesus-app` (manages Auth0, Tigris, Cloudflare, Render). Secrets read via SOPS — see [Secrets Management](#secrets-management--sops--age).

---

## Known Issues / Handoff Notes (as of 2026-07-19)

- **No CI test gate:** `go test`/lint never run in the deploy pipeline.
- **Many unmerged feature branches** (27 as of this cleanup, since pruned to the actively-in-progress ones) exist alongside `main` — some may be superseded by items already checked off in `BACKLOG.md`; triage before resuming.
- `docs/code-review-2026-03-15.md`'s HIGH-severity open-redirect finding (unvalidated `cja_redirect` cookie) was fixed the same day in commit `4dbd6a0` — the doc itself is stale on that point, don't take its findings at face value without checking current code first.
- **USPS Addresses API deprecation:** `internal/usps/client.go` calls `/addresses/v3/address` against Addresses API **3.2.3** (`docs/usps/addresses-v3r2_3.yaml`). USPS is retiring 3.2.3 in favor of 3.3.1 — customers who hadn't completed onboarding/signed the license agreement by **July 12, 2026** risk service interruptions starting that date; full 3.3.1 access opens **August 1, 2026**. As of this handoff, no migration work has been done. Address validation on need submission may already be degraded — check USPS onboarding status and plan the 3.3.1 migration early.
