# ChristJesus.app — Copilot Agent Instructions

## Big Picture
- Stack: Go SSR app (`html/template`) + Flow router + PostgreSQL + Tailwind utility classes.
- Entry point: `cmd/christjesus/serve.go` wires AWS clients, DB pool, repositories, JWK cache, then builds `server.Service`.
- Server composition: `internal/server/server.go` owns middleware, routes, template loading, and static assets.
- Data access is repository-based in `internal/store/*.go` (Squirrel SQL + `pgxscan`).

## Request + Auth Flow
- Global middleware order: `StripTrailingSlash` → `LoggingMiddleware` → `AttachAuthContext`.
- `AttachAuthContext` reads encrypted cookie, validates Cognito JWT, and stores `user_id`/`email` in request context.
- `RequireAuth` is used only on protected route groups and expects auth context to already be attached.
- Login/logout + redirect-intent cookies live in `internal/server/auth.go`.

## Template + Navbar Pattern (Important)
- Always render with `s.renderTemplate(w, r, templateName, data)` (`internal/server/render.go`).
- Page data structs live in `pkg/types/page_data.go` and should embed `types.BasePageData`.
- Navbar state is injected through interface: `types.NavbarDataSetter` via `BasePageData.SetNavbarData`.
- `component.nav` expects `.Navbar.*` fields (see `internal/server/templates/components/nav.html`).
- Avoid `map[string]any` page payloads for template rendering; use typed page data structs.

## Routing + Handler Conventions
- Use method-specific routes with Flow: `r.HandleFunc(path, handler, http.MethodGet|Post)`.
- Naming pattern: `handleGetX` / `handlePostX`.
- For forms, use PRG with `http.StatusSeeOther` after successful POST.
- Need onboarding routes are in `internal/server/onboarding.go` and currently represent the most complete vertical slice.

## Data + Schema Conventions
- DB search path is forced to `christjesus` in `internal/db/postgres.go`.
- Atlas manages only `christjesus` schema (`migrations/atlas.hcl`).
- Current need/location model: `needs.user_address_id` + `needs.uses_non_primary_address` and separate `user_addresses` table.
- Repositories commonly use `utils.StructTagValues(...)` for column lists and `utils.StructToMap(...)` for inserts/updates.

## Frontend Conventions
- Templates are embedded and parsed recursively from `internal/server/templates/` by define-name.
- Layout wrapper is `header/footer` in `internal/server/templates/layout.html`.
- Keep Tailwind class usage consistent with existing tokens (`static/tokens.css`); avoid introducing ad-hoc style systems.

## Developer Workflows
- Run app: `just serve`
- Build binary: `just build`
- Tests: `just test`
- Format: `just fmt`
- Migration preview/apply: `just migrate-plan` / `just migrate`
- Seed lookup data: `just seed`
- Hot reload (if installed): `just dev` (uses `air`)

## Config + Integrations
- Config is loaded from env vars with no prefix (`envconfig.Process("", c)`).
- Required envs include: `DATABASE_URL`, Cognito (`COGNITO_*`), S3 (`S3_BUCKET_NAME`), and cookie encryption keys.
- External dependencies: AWS Cognito (auth), AWS S3 (document storage), PostgreSQL (app data).
