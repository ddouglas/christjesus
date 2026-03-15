# ChristJesus.app — Third-Party Code Review
**Date:** 2026-03-15  
**Reviewer:** GitHub Copilot (Claude Sonnet 4.6)  
**Scope:** Full repository review — security, architecture, code quality, test coverage  
**Repo age:** ~3 weeks (alpha-stage)

---

## Overall Impression

The codebase has a solid structural foundation. The repository/service split is clean, the route naming convention is disciplined, the auth flow (PKCE-adjacent with nonce/state) is correctly implemented for the most part, and the use of typed page-data structs over `map[string]any` is a good call. The onboarding flow reads like a genuine vertical slice — it's the most finished part of the app.

That said, three weeks of AI-assisted iteration has deposited some technical debt that should be addressed before the alpha grows much further. The issues below are ranked: **Security** first, then **Architecture**, then **Code Quality**, then **Testing**.

---

## 1. Security

### 1.1 Open Redirect via Unvalidated Redirect Cookie [HIGH]

**File:** `internal/server/auth.go` — `handleGetAuthCallback` + `setRedirectCookie`

After a successful login, the app reads the plaintext `cja_redirect` cookie and redirects the user to it:

```go
redirectCookie, err := r.Cookie(internal.COOKIE_REDIRECT_NAME)
if err == nil {
    path := redirectCookie.Value
    s.clearRedirectCookie(w)
    http.Redirect(w, r, path, http.StatusSeeOther)
    return
}
```

This cookie is set **unencrypted** (plain `http.SetCookie`, not `s.cookie.Encode`). An attacker who can plant a cookie for the domain (via a subdomain takeover, a same-site XSS, or a cookie injection vulnerability elsewhere) can redirect a successfully-authenticated user to an arbitrary external URL.

**Fix:** Validate that the path is host-relative before using it. A simple guard:

```go
if path := redirectCookie.Value; strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "//") {
    s.clearRedirectCookie(w)
    http.Redirect(w, r, path, http.StatusSeeOther)
    return
}
s.clearRedirectCookie(w)
// fall through to home
```

Or encode the redirect cookie with `s.cookie.Encode` the same way state/nonce are protected.

---

### 1.2 No Security Response Headers [MEDIUM]

No middleware sets standard defensive HTTP headers. Every page response is missing:

- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: SAMEORIGIN` (or `CSP: frame-ancestors 'self'`)
- `Referrer-Policy: strict-origin-when-cross-origin`
- `Content-Security-Policy` (even a loose starter policy)

For an app that handles financial donations and personally sensitive need stories, clickjacking and MIME-sniffing protection should be baseline. Add a single middleware that sets these headers before any response is written.

---

### 1.3 No CSRF Protection on State-Mutating Forms [MEDIUM]

The auth flow correctly uses a random `state` parameter. But the application's own forms — donation submission, profile-need messaging, status transitions (`set-ready`, `pull-back`) — carry no CSRF token.

`SameSite=Lax` cookies provide partial protection: they block cross-site `<form>` submissions triggered by navigation. However they do **not** protect against:
- Same-site subdomains (if any)
- Any form that uses `fetch`/XHR initiated from the same origin (which you may add later with HTMX or similar)

Before adding any frontend interactivity layer, add per-session or per-form CSRF tokens. The `gorilla/csrf` or a simple double-submit cookie pattern both work well here.

---

### 1.4 Debug Library Shipped in Production Binary [LOW]

`internal/pp.go` is a blank import:

```go
package internal
import _ "github.com/k0kubun/pp/v3"
```

This pulls the `k0kubun/pp` pretty-printer into the compiled binary for no production purpose. Remove `internal/pp.go` and drop the dependency from `go.mod`. If it's actively useful during development, use a build tag (`//go:build dev`) to keep it out of production builds.

---

## 2. Architecture

### 2.1 `server.New()` Has 18 Parameters [MEDIUM]

```go
func New(
    config *types.Config,
    logger *logrus.Logger,
    s3Client *s3.Client,
    stripeClient *stripe.Client,
    needsRepo *store.NeedRepository,
    progressRepo *store.NeedProgressRepository,
    ...12 more...
) (*Service, error)
```

Every new integration or repository requires touching the call-site in `cmd/christjesus/serve.go` and the function signature. Use an options struct:

```go
type ServiceOptions struct {
    Config     *types.Config
    Logger     *logrus.Logger
    S3Client   *s3.Client
    // ...
}

func New(opts ServiceOptions) (*Service, error) { ... }
```

This is a low-effort change with high readability payoff, especially as more repos get added.

---

### 2.2 No Pagination on `BrowseNeeds` [MEDIUM]

`internal/store/need.go` — `BrowseNeeds`:

```go
func (r *NeedRepository) BrowseNeeds(ctx context.Context) ([]*types.Need, error) {
    query, args, err := psql().Select(needColumns...).From(needTableName).
        Where(sq.Eq{"status": []types.NeedStatus{types.NeedStatusActive, types.NeedStatusFunded}}).
        Where(sq.Eq{"deleted_at": nil}).
        OrderBy("created_at desc").
        ToSql()
```

There is no `LIMIT` or `OFFSET`. As soon as there are more than a few hundred active needs, this query will slow significantly and could OOM the Go process (every row loads a full `types.Need` struct into memory). The explore/admin path already has correct pagination. The browse path needs the same treatment. Add cursor or offset pagination here before alpha opens to the public.

---

### 2.3 Dual Source-of-Truth for `amount_raised_cents` [MEDIUM]

The `needs` table has an `amount_raised_cents` column. It is never written after the initial zero-value insert. The actual raised amount is computed from `donation_intents` via `applyFinalizedRaisedAmounts`, which queries the DB on every page render that shows needs (home, browse, profile, admin explorer).

This creates two problems:
1. The `needs.amount_raised_cents` column exists but is always stale — it's misleading to anyone reading the schema or query results directly.
2. Every browse/home render fires a separate aggregate query against `donation_intents`.

**Options (pick one):**
- **Keep the column, keep it accurate:** Update `amount_raised_cents` in `donation_intents.FinalizeIntentByID` using a `UPDATE needs SET amount_raised_cents = (SELECT SUM...) WHERE id = $1`. Then the column is authoritative and you don't need `applyFinalizedRaisedAmounts` at render time.
- **Remove the column:** Drop `amount_raised_cents` from `needs` entirely. Accept that it's always computed. Add a DB-level view or materialized view for performance.

The current hybrid approach is the worst of both worlds.

---

### 2.4 Config Has Redundant Auth Fields [LOW]

`pkg/types/config.go` defines both:
```go
Auth0Domain    string `envconfig:"AUTH0_DOMAIN"`
Auth0ClientID  string `envconfig:"AUTH0_CLIENT_ID"`
AuthIssuerURL  string `envconfig:"AUTH_ISSUER_URL"`   // programmatically set from Auth0Domain
AuthClientID   string `envconfig:"AUTH_CLIENT_ID"`   // programmatically set from Auth0ClientID
```

`AuthIssuerURL` and `AuthClientID` are computed in `loadConfig()` and never read from env directly (if they were provided via env they'd be overwritten). They could just be unexported struct fields set at config load time, or the duplication could be collapsed. As-is, the struct implies they are user-configurable env vars when they aren't.

---

### 2.5 Database Has No Foreign Key Constraints [LOW]

The HCL migration files document relationships in comments:
```hcl
column "user_id" {
  comment = "References christjesus.users(id)"
}
```

But no `reference` blocks or FK constraints exist. This means the DB will happily store phantom references (e.g., a `need` pointing to a deleted `user_id`). For an app in early alpha with a small dataset, this is acceptable short-term, but FK constraints should be added as migrations before the data layer grows. Atlas HCL fully supports `reference` blocks.

---

## 3. Code Quality

### 3.1 N+1 Query Pattern in `handleGetProfile` [HIGH]

`internal/server/profile.go`:

```go
for _, need := range needs {
    assignments, err := s.needCategoryAssignmentsRepo.GetAssignmentsByNeedID(ctx, need.ID)
    // ...
    for _, assignment := range assignments {
        if !assignment.IsPrimary { continue }
        category, err := s.categoryRepo.CategoryByID(ctx, assignment.CategoryID)
        // ...
    }
}
```

For a user with N needs, this fires `N` assignment queries and up to `N` additional category queries. The fix is to collect all need IDs, do a single `GetAssignmentsByNeedIDs`, collect all unique category IDs, and do a single `CategoriesByIDs`. Both of those batch methods already exist — they just aren't used on this path.

---

### 3.2 Error Sentinel Comparison Without `errors.Is` [MEDIUM]

10+ occurrences of:
```go
if err == types.ErrNeedNotFound {
```

The Go idiom since 1.13 is `errors.Is(err, types.ErrNeedNotFound)`. Today it works because `types.ErrNeedNotFound = fmt.Errorf("need not found")` is a plain (non-wrapping) error. But the moment someone wraps it with `fmt.Errorf("context: %w", types.ErrNeedNotFound)` (common in repo methods), the equality check will silently fail and the not-found path will stop being taken.

Use `errors.Is` consistently. This is a global find-replace.

---

### 3.3 `forms.go` Is Pure Dead Code [LOW]

`internal/server/forms.go` contains nothing but a large block comment of commented-out handlers. It should either be deleted or the functionality should be tracked in a GitHub issue/BACKLOG.md entry. Keeping inert files in the source tree creates noise.

---

### 3.4 Inconsistent Path Parameter Naming [LOW]

Admin handlers extract the need ID as `:id`:
```go
r.HandleFunc(RoutePattern(RouteAdminNeedReview), ...) // pattern: /admin/needs/:id
needID := strings.TrimSpace(r.PathValue("id"))
```

Profile and onboarding handlers use `:needID`:
```go
needID := strings.TrimSpace(r.PathValue("needID"))
```

Pick one convention. `:needID` is more descriptive and should be adopted everywhere. Changing the admin routes also makes handlers more grep-friendly.

---

### 3.5 `internalServerError` Writes Status After Body Might Have Started [LOW]

There's no visible `internalServerError` definition in the files I read (it may be in `server.go` beyond what was visible). One pattern to watch: if any code writes to `w` before calling `s.internalServerError(w)`, the 500 status code will be silently ignored because the header was already sent. This is especially risky in handlers that log to `w` (e.g., setting Content-Type) before an error occurs.

Verify that `internalServerError` uses `http.Error` (which resets the body) and that no handler writes partial output before calling it.

---

### 3.6 `profile_need_edit.go` is a God File [LOW]

At 1000+ lines, `profile_need_edit.go` handles all of: location editing, category editing, story editing, document management, document metadata, document deletion, and the review step. Each of these is a distinct sub-flow with its own GET/POST pair. They should be split into `profile_need_edit_location.go`, `profile_need_edit_categories.go`, etc. The onboarding file has the same problem.

This doesn't affect correctness but significantly harms navigability.

---

### 3.7 `BrowseFilters.VerificationIDs` References a Non-Existent Concept [LOW]

`pkg/types/page_data.go`:
```go
type BrowseFilters struct {
    VerificationIDs map[string]bool
    // ...
}
```

There is no "verification" concept in the need type or category system. This field appears to be a placeholder from a planned feature. Add a comment explaining what it's for or remove it until the feature is designed.

---

### 3.8 `handleGetProfile` Makes a DB Call Inside the Response Loop [MEDIUM]

Beyond the N+1 noted in §3.1, the profile handler also calls `s.donationIntentRepo.DonationIntentsByDonorUserID` and then `s.needsRepo.NeedsByIDs` to build donation summaries. This is fine as a batch, but note that the need-summary building loop also calls `s.needCategoryAssignmentsRepo.GetAssignmentsByNeedID` inside the `for _, need := range needs` — each category fetch happens individually. Batch it all.

---

## 4. Test Coverage

### 4.1 Store Layer Is Completely Untested [HIGH]

There are no tests for any file in `internal/store/`. The store layer is the deepest critical path in the app — it translates business intent into SQL. A bug in `UpsertIdentity`, `FinalizeIntentByID`, or `ModerationActionsByNeed` would cause silent data corruption or silent no-ops.

At minimum, write integration tests against a test database (can use `testcontainers-go` or a local Postgres). The justfile already has `just test` — wire up a test DB URL via env.

---

### 4.2 Handler Tests Are Unit Tests Only (No HTTP Integration) [MEDIUM]

The existing tests in `middleware_test.go`, `auth_test.go`, and `routes_test.go` are good unit tests of isolated functions. But no test exercises an actual HTTP round-trip through the full middleware stack (`StripTrailingSlash` → `LoggingMiddleware` → `AttachAuthContext` → `RequireAuth` → handler). A table-driven integration test suite using `httptest.NewServer` and a real DB connection would catch regressions that unit tests can't see (e.g., a handler returning 500 because it was wired to the wrong route).

---

### 4.3 Onboarding Flow Has Zero Test Coverage [MEDIUM]

The onboarding flow is called out in the copilot instructions as "the most complete vertical slice," yet `internal/server/onboarding.go` has no corresponding test file. The step-routing logic (`redirectNeedOnboarding`), need creation, and step advancement are all untested. Given this is the primary user entry point, a broken step would be invisible until a real user hits it.

---

## 5. Minor Observations

- **`reconcile_donations.go` is a well-structured CLI tool** — good instinct to separate reconciliation into a CLI command rather than an in-process goroutine. No issues here.

- **`WithTx` helper is clean** — the deferred-rollback pattern in `store/tx.go` is correct Go, including the check for `pgx.ErrTxClosed`. Good.

- **Logrus structured logging is used consistently** — field-level context on errors (`WithField("need_id", ...)`) makes log tracing practical. Keep this pattern.

- **`subtleCompare` is correctly implemented** — the constant-time comparison for state/nonce is right. The length check before the XOR loop does not introduce a timing oracle because length mismatch is not a secret for these values (both state and nonce are randomly generated by the server).

- **`ModerationQueueNeeds` has a hardcoded 500-item limit** — `ModerationQueueNeedsPage(ctx, 1, 500)` is called when loading the admin moderation queue. This will silently cap the visible queue at 500 items. Use the paginated variant throughout.

---

## Summary Table

| # | Area | Issue | Severity |
|---|------|-------|----------|
| 1.1 | Security | Open redirect via unvalidated redirect cookie | High |
| 1.2 | Security | No security response headers | Medium |
| 1.3 | Security | No CSRF tokens on state-mutating forms | Medium |
| 1.4 | Security | Debug library in production binary | Low |
| 2.1 | Architecture | 18-param constructor | Medium |
| 2.2 | Architecture | No pagination on `BrowseNeeds` | Medium |
| 2.3 | Architecture | Dual source-of-truth for raised amount | Medium |
| 2.4 | Architecture | Redundant auth config fields | Low |
| 2.5 | Architecture | No DB foreign key constraints | Low |
| 3.1 | Code Quality | N+1 query in `handleGetProfile` | High |
| 3.2 | Code Quality | `err ==` instead of `errors.Is` | Medium |
| 3.3 | Code Quality | `forms.go` is dead code | Low |
| 3.4 | Code Quality | Inconsistent path param naming | Low |
| 3.5 | Code Quality | Potential double-write on error | Low |
| 3.6 | Code Quality | God files (edit + onboarding) | Low |
| 3.7 | Code Quality | `VerificationIDs` references nothing | Low |
| 3.8 | Code Quality | Additional N+1 in donation summary | Medium |
| 4.1 | Testing | Store layer entirely untested | High |
| 4.2 | Testing | No HTTP integration tests | Medium |
| 4.3 | Testing | Onboarding flow untested | Medium |

---

## Recommended Priority Order

1. **1.1** — Fix the open redirect. One-liner validation, high security value.
2. **3.1 + 3.8** — Fix the N+1 queries on the profile page before user load grows.
3. **2.2** — Add pagination to `BrowseNeeds`.
4. **4.1** — Start store-layer integration tests. Even 3–4 tests for the critical paths (upsert identity, finalize intent) will catch regressions.
5. **1.2** — Add security headers middleware (single function, applied globally).
6. **3.2** — Global `errors.Is` sweep (mechanical fix).
7. **2.3** — Resolve the raised-amount dual source-of-truth.
8. **1.4** — Remove `internal/pp.go`.
9. **2.1** — Refactor `server.New()` to accept an options struct.
10. **1.3** — CSRF tokens before adding any frontend JS that posts forms.
