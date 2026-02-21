# ChristJesus.app - AI Coding Agent Instructions

## Architecture Overview

**Stack:** Go 1.21+ SSR with HTMX, PostgreSQL, Tailwind CSS  
**Router:** `alexedwards/flow` - method-specific routing with `r.HandleFunc(path, handler, method)`  
**Templates:** `html/template` with `embed.FS` - all templates live in `internal/server/templates/`  
**Database:** `pgxpool` + `Masterminds/squirrel` for query building, `scany/v2` for scanning  
**Styling:** Tailwind CSS + custom CSS variables in `static/tokens.css`

## Template Conventions

### Naming Pattern
- **Pages:** `{{define "page.{resource}[.{subresource}]"}}` → Maps to folder: `templates/pages/{resource}/`
- **Components:** `{{define "component.{name}"}}` → Lives in `templates/components/{name}.html`
- **Layout:** `{{template "header" .}}` and `{{template "footer"}}` wrap all pages (defined in `layout.html`)

**Examples:**
- `page.home` → `templates/pages/home.html`
- `page.onboarding.need.location` → `templates/pages/onboarding/need/location.html`
- `component.need.card` → `templates/components/need-card.html`

### Template Data Requirements
All page templates **must** receive data with a `Title` field (used by header). Pass `nil` only if page doesn't need dynamic data AND doesn't use header template.

### Template Loading
Templates are loaded via `fs.WalkDir` on embedded FS at startup. The loader parses ALL `.html` files in `templates/` recursively and defines them by their `{{define}}` names, not file paths.

## Routing Patterns

### Flow Router Usage
```go
// Flow router: path, handler, HTTP method(s)
r.HandleFunc("/browse", s.handleBrowse, http.MethodGet)
r.HandleFunc("/onboarding/need/location", s.handleGetOnboardingNeedLocation, http.MethodGet)
r.HandleFunc("/onboarding/need/location", s.handlePostOnboardingNeedLocation, http.MethodPost)
```

**Handler naming convention:**
- `handleGet{Resource}{Action}` for GET requests
- `handlePost{Resource}{Action}` for POST requests
- `handle{Resource}` for simple GET-only pages

### Form Submission Pattern
**Always use POST-Redirect-GET (PRG):**
1. Form action points to **current page URL** (not next step)
2. POST handler validates/saves data
3. Redirect with `http.StatusSeeOther` (303) to next page
4. Prevents duplicate submissions, enables browser back button

**Example:**
```html
<!-- location.html form posts to ITSELF -->
<form method="POST" action="/onboarding/need/location">
```
```go
func (s *Service) handlePostOnboardingNeedLocation(w http.ResponseWriter, r *http.Request) {
    // Validate form data
    // Save to session or DB
    http.Redirect(w, r, "/onboarding/need/categories", http.StatusSeeOther)
}
```

## Styling Conventions

### Container Pattern
**All page sections use consistent max-width containers:**
```html
<div class="mx-auto w-full max-w-6xl px-4 py-12 md:px-6">
  <!-- content -->
</div>
```

### CSS Custom Properties
Defined in `static/tokens.css`, used via Tailwind arbitrary values:
- Primary: `bg-[color:var(--cj-primary)]` → `#1b3a52`
- Secondary: `bg-[color:var(--cj-secondary)]` → `#e6a944`
- Accent: `bg-[color:var(--cj-accent)]` → `#4a9b9f`

**Pattern:** Always use full Tailwind classes (not custom `.btn` classes). Maintain consistency with existing components.

### Fonts
- **Body:** Inter (400, 500, 600, 700)
- **Headings:** Poppins (500, 600, 700)
- **Accent:** Montserrat (500, 600, 700)

Loaded via Google Fonts CDN in `layout.html`.

## Database Patterns

### Repository Layer
One file per table in `internal/store/{resource}.go`:
```go
const todoTableName = "todos"
var todoTableColumns = []string{"id", "title", "completed", "created_at"}

func (r *Repository) GetTodoByID(ctx context.Context, id string) (*types.Todo, error) {
    query, args, _ := psql().
        Select(todoTableColumns...).
        From(todoTableName).
        Where(sq.Eq{"id": id}).
        ToSql()
    
    var todo types.Todo
    err := pgxscan.Get(ctx, r.pool, &todo, query, args...)
    return &todo, err
}
```

### Query Building Helper
`psql()` in `store/stmt.go` returns `sq.StatementBuilder` with Dollar placeholders (PostgreSQL format).

## Development Workflow

### Common Commands (justfile)
```bash
just serve         # Run dev server
just build         # Build binary to bin/
just migrate-plan  # Preview Atlas migrations (dry-run)
just migrate       # Apply Atlas migrations
just fmt           # Format Go code
```

### Configuration
Uses `envconfig` with `APP_` prefix by default. Required vars:
- `APP_DATABASE_URL` - PostgreSQL connection string
- `APP_SERVER_PORT` - Default 8080

See `pkg/types/config.go` for full config struct.

## Onboarding Architecture

**Multi-step flows with session-based progress tracking (planned):**

1. **Need Flow** (8 steps): welcome → location → categories → details → story → documents → review → confirmation
2. **Donor Flow** (2 steps): welcome → preferences
3. **Sponsor Flow** (incomplete): Individual vs Organization selection, then multi-step flows

**Handler Organization:**
- `internal/server/onboarding.go` - All onboarding handlers
- `internal/server/register.go` - Registration entry points
- `internal/server/pages.go` - Marketing/browse pages

## Authentication (Planned)

**Approach:** AWS Cognito OAuth with session management
- OAuth callback handler sets httpOnly session cookie
- Middleware verifies session on protected routes
- User profile stored in local DB, linked via `cognito_id`

**Not yet implemented** - currently all pages accessible without auth.

## Critical Files

- `SCAFFOLDING_GUIDE.md` - Comprehensive architecture reference (1500+ lines)
- `internal/server/server.go` - Server initialization, router setup, template loading
- `internal/server/middleware.go` - Logging middleware pattern
- `pkg/types/` - All domain models with `db:"column"` tags for scany
- `migrations/*.pg.hcl` - Atlas declarative schemas (not sequential migrations)

## Working Boundaries

**User owns:** Backend logic (handlers, validation, business rules, database queries)  
**AI assists with:** Templates, CSS/styling, HTML structure, boilerplate scaffolding

When generating handlers, provide simple pass-through implementations for user to enhance with validation/logic.

## Active TODO List

### Auth & Sessions (High Priority)
- [ ] Set up AWS Cognito user pool via IaC (Terraform/CloudFormation)
- [ ] Create `internal/auth/cognito.go` - Token exchange, JWT verification
- [ ] Create `internal/auth/middleware.go` - Session validation, RequireAuth middleware
- [ ] Create `internal/auth/session.go` - Session management helpers
- [ ] Add login/register templates with OAuth provider buttons
- [ ] Implement `/auth/callback` handler - verify JWT, set cookie, redirect
- [ ] Database migrations: `users` table (id, cognito_id, email, name, role, onboarding_complete)
- [ ] Database migrations: `sessions` table (id, user_id, expires_at) OR Redis setup
- [ ] Protect onboarding routes with RequireAuth middleware
- [ ] Update config.go with Cognito settings (region, user pool ID, client ID)

### Data Models & Database
- [ ] Create `needs` table - Store completed need posts
- [ ] Create `need_drafts` table - Store in-progress onboarding (or use sessions)
- [ ] Create `donor_preferences` table - Categories, location, donation range
- [ ] Create `categories` table - Replace sampleCategories() with real data
- [ ] Create `locations` table - City/state data for filtering
- [ ] Define types in `pkg/types/` for all new tables
- [ ] Create repository files in `internal/store/` for each table

### Onboarding Enhancements
- [ ] Add form validation to all POST handlers (location, categories, details, etc.)
- [ ] Implement draft saving - store partial data during onboarding flow
- [ ] Implement draft loading - pre-fill forms on revisit
- [ ] File upload handling for documents step (S3 integration)
- [ ] Complete sponsor individual onboarding flow (5+ steps)
- [ ] Complete sponsor organization onboarding flow (5+ steps)
- [ ] Add edit functionality to review page
- [ ] Wire up confirmation page actions (browse needs, submit another)

### HTMX Features (Deferred)
- [ ] Implement HTMX for browse page filters (dynamic need filtering)
- [ ] Implement HTMX for donation modals
- [ ] Add HTMX partial renders for form validation errors
- [ ] Add HTMX infinite scroll for browse page

### Additional Pages
- [ ] Create need detail page with full information
- [ ] Create categories page showing all categories
- [ ] Create map view of needs
- [ ] Create guidelines/how-it-works page

### Infrastructure
- [ ] Set up S3 bucket for document uploads
- [ ] Configure S3 signed URLs for secure uploads
- [ ] Set up email service for notifications (SES?)
- [ ] Production deployment configuration

### Pending Decisions (Requires Project Owner Discussion)
- [ ] **Multi-tenancy:** Can one user have multiple roles (e.g., both donor AND have a need)?
- [ ] **Search/Filter/Recommendations:** SSR vs client-side trade-offs, HTMX implementation strategy for need discovery
- [ ] **Need Verification:** Workflow and criteria for verifying needs (gold/silver/bronze levels)
- [ ] **Notification System:** Email triggers (donations received, nearby needs posted, etc.) and delivery mechanism
