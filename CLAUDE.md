# CLAUDE.md — christjesus

## Project Overview

A Go web application for managing charitable needs. Server-side rendered (SSR) with PostgreSQL, Auth0 authentication, Tigris object storage, Stripe payments, and Fly.io hosting.

- **Module:** `christjesus`
- **Go version:** 1.24.0
- **Task runner:** `just` (justfile), which auto-loads `.env`

---

## Running Locally

```bash
# 1. Start the local Postgres instance
docker compose up -d

# 2. First time only — copy and fill in env vars
cp .env.example .env
# Edit .env with your Auth0, Stripe, Tigris, and DB credentials

# 3. Apply schema migrations
just migrate

# 4. (Optional) Seed categories and fake data
just seed

# 5. Start with hot reload (recommended)
just dev
```

The app runs on `http://localhost:8080`. `just dev` uses `air` for hot reload and watches `.go`, `.html`, `.css`, `.js`, and `.tpl` files.

Alternative startup commands:
```bash
just serve          # go run ./cmd/christjesus serve
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

---

## Environment Variables

The justfile loads `.env` automatically. Create `.env` from `.env.example`.

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

> **Note:** The `.env.example` file has stale `COGNITO_*` variables — ignore those. The active config uses `AUTH0_*` variables defined in `pkg/types/config.go`.

---

## Database — Supabase / PostgreSQL

- **Production:** Supabase-hosted PostgreSQL (connection string in `DATABASE_URL`)
- **Local dev:** Docker Compose runs `postgres:18-alpine` on `localhost:5432` (user/pass/db all `christjesus`)
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
auth0 users search --query "*" --number 100 \
  | jq -r '.[].user_id' \
  | xargs -I {} auth0 users delete {}
```

---

## Container Platform — Fly.io

- **App name:** `christjesus`
- **Region:** `iad` (US East / Virginia)
- **Internal port:** `8080`
- **Config:** `fly.toml`

Machines auto-stop when idle and auto-start on request (`min_machines_running = 0`). VM size is 1 shared CPU / 256MB RAM.

**Deploying:**
```bash
fly deploy
```

**Secrets** must be set via `fly secrets set` — never committed to the repo. This includes `DATABASE_URL`, `AUTH0_CLIENT_SECRET`, `COOKIE_HASH_KEY`, `COOKIE_BLOCK_KEY`, Stripe keys, and Tigris keys.

The Dockerfile is a two-stage build:
1. `golang:1.26-alpine` — builds binary with `CGO_ENABLED=0 GOOS=linux`
2. `alpine:3.21` — minimal runtime image, runs `christjesus serve`

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
terraform/            # Auth0 and Tigris infrastructure (Terraform)
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
- **Tests:** Standard library `testing` only — no testify or gomock
- **Formatting:** `go fmt` (no golangci-lint configured)

---

## Infrastructure

- **Object storage:** Tigris (S3-compatible) — not Supabase Storage. `internal/storage/supabase.go` is a legacy file and not active.
- **Payments:** Stripe (donation intents + webhooks)
- **IaC:** Terraform Cloud, org `christjesus`, workspace `christjesus-app` (manages Auth0 + Tigris)
