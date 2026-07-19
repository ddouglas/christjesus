# Body of Christ

A Go web application for connecting donors with people in need. Server-side rendered, backed by PostgreSQL, Auth0, Tigris object storage, and Stripe.

Live at [bodyofchrist.app](https://bodyofchrist.app).

## Quick start

```bash
# 1. Get the local dev SOPS/age key from a project maintainer, place it at ~/.age/key.txt
#    (see CLAUDE.md > Secrets Management for how this works)

# 2. Apply schema migrations
sops exec-env configs/local/app.enc.yaml 'just migrate'

# 3. (Optional) Seed categories and fake data
sops exec-env configs/local/app.enc.yaml 'just seed'

# 4. Start with hot reload
just dev
```

The app runs on `http://localhost:8080`. Prefer Docker? Run `just docker-up` instead of step 4 — same secrets, containerized.

Local dev talks to a shared remote Supabase database by default, not a local Postgres. `docker compose up -d` starts a local Postgres instance if you want one instead (see CLAUDE.md for how to point the app at it).

## Documentation

- **[CLAUDE.md](CLAUDE.md)** — the primary reference for this codebase: architecture, environment variables, database, auth, secrets management, deployment, and project conventions. Written for AI coding assistants but equally useful for humans; start there.
- **[docs/adr/](docs/adr/)** — architecture decision records.
- **[docs/](docs/)** — design docs, QA plans, and other working notes.
- **[BACKLOG.md](BACKLOG.md)** — open work and implementation notes.

## Stack

Go · PostgreSQL (Supabase) · Auth0 · Tigris (S3-compatible storage) · Stripe · Resend · Render · Cloudflare · Terraform (HCP Terraform / Terraform Cloud) · SOPS + age

## Tests

```bash
just test
```
