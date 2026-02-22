# ChristJesus.app - AI Coding Agent Instructions

## Current Status (Last Updated: Feb 21, 2026)

**✅ Completed:**
- Supabase Auth integration with JWT verification (jwx v3)
- Cookie-based redirect intent flow for authentication
- Protected routes with RequireAuth middleware
- Onboarding path selection page (Need/Donor/Sponsor)
- Complete database schema design (4 tables: needs, categories, assignments, documents)
- Atlas migration configuration (only manages christjesus schema)

**🔄 In Progress:**
- Database migration ready to apply (35 statements validated)
- Need flow templates created (awaiting data persistence layer)

**⏭️ Next Steps:**
- Apply Atlas migration to create tables
- Seed initial category data
- Create Go types and repository layer
- Wire POST handlers to persist onboarding data

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

**Multi-step flows with database-based progress tracking:**

1. **Path Selection** (`/onboarding`): Three-card selection page for Need, Donor, or Sponsor roles
2. **Need Flow** (8 steps): welcome → location → categories → details → story → documents → review → confirmation
3. **Donor Flow** (2 steps): welcome → preferences
4. **Sponsor Flow** (incomplete): Individual vs Organization selection, then multi-step flows

**Handler Organization:**
- `internal/server/onboarding.go` - All onboarding handlers including path selection
- `internal/server/register.go` - Registration entry points
- `internal/server/pages.go` - Marketing/browse pages
- `internal/server/auth.go` - Login/logout handlers with redirect intent

**Route Protection:**
- All `/onboarding/*` routes protected by `RequireAuth` middleware
- Unauthenticated users redirected to login with return intent stored in cookie

## Authentication

**Implementation:** Supabase Auth with JWT verification
- **JWT Library:** `lestrrat-go/jwx v3` with ES256 algorithm
- **JWK Fetching:** Downloads from `https://{supabase-project}.supabase.co/auth/v1/jwks`
- **Token Storage:** Encrypted httpOnly cookie (`cja_access_token`) via `gorilla/securecookie`
- **Middleware:** `RequireAuth` verifies JWT signature, validates claims, adds user context

**Redirect Intent Flow:**
- Cookie-based storage (`cja_redirect_after_login`) with 5-minute TTL
- Set by middleware when auth required but user not authenticated
- Cleared after successful login, redirects user to original destination
- Helper functions: `setRedirectCookie()`, `getAndClearRedirectCookie()`, `clearRedirectCookie()`

**Context Keys:**
- Custom `contextKey` type prevents collisions
- `contextKeyUserID`: User ID from JWT "sub" claim (UUID string)
- `contextKeyEmail`: Email from JWT custom claim (string)

**Access Pattern:**
```go
userID := r.Context().Value(contextKeyUserID).(string)
email := r.Context().Value(contextKeyEmail).(string)
```

## Database Schema

**Database:** PostgreSQL via Supabase (port 6543 pooled connection)
**Schema:** `christjesus` - Custom schema separate from Supabase's `public`
**Migration Tool:** Atlas with HCL declarative schemas

**Tables:**
1. **`needs`** - All need posts with status-based lifecycle
   - Status flow: `draft` → `submitted` → `pending_review` → `approved`/`rejected` → `active` → `funded` → `closed`
   - Progress tracking: `current_step`, `completed_steps` (jsonb array)
   - Money stored as integers: `amount_needed_cents`, `amount_raised_cents`
   - Nullable fields required when status=submitted: city, state, title, amount, story, etc.
   - User reference: `user_id` UUID (no FK due to cross-schema, references `auth.users`)
   
2. **`need_categories`** - Category lookup table
   - Fields: id, name, slug, description, icon, display_order, is_active
   - Unique constraint on slug
   
3. **`need_category_assignments`** - Many-to-many junction
   - Composite PK: (need_id, category_id)
   - CASCADE deletes on both FKs
   
4. **`need_documents`** - File uploads via Supabase Storage
   - Fields: id, need_id, user_id, document_type, file_name, file_size_bytes, mime_type, storage_key
   - `storage_key` references Supabase Storage bucket path
   - CASCADE delete on need_id FK

**Atlas Configuration:**
- `atlas.hcl` configured with `schemas = ["christjesus"]` to only manage app schema
- Protects all Supabase schemas (auth, storage, public, extensions, etc.)
- Migration commands: `just migrate-plan` (dry-run), `just migrate` (apply)

## Critical Files

- `SCAFFOLDING_GUIDE.md` - Comprehensive architecture reference (1500+ lines)
- `internal/server/server.go` - Server initialization, router setup, template loading
- `internal/server/middleware.go` - Authentication (RequireAuth) and logging middleware
- `internal/server/auth.go` - Login/logout handlers with redirect intent
- `internal/const.go` - Application constants (cookie names)
- `pkg/types/` - All domain models with `db:"column"` tags for scany
- `migrations/schema.pg.hcl` - Atlas declarative schema for christjesus database
- `migrations/atlas.hcl` - Atlas configuration (only manages christjesus schema)

## Working Boundaries

**User owns:** Backend logic (handlers, validation, business rules, database queries)  
**AI assists with:** Templates, CSS/styling, HTML structure, boilerplate scaffolding

When generating handlers, provide simple pass-through implementations for user to enhance with validation/logic.

## Active TODO List

### Auth & Sessions
- [x] JWT verification middleware with jwx v3 and ES256
- [x] RequireAuth middleware with context key pattern
- [x] Cookie-based redirect intent flow (5-minute TTL)
- [x] Login/register handlers with redirect support
- [x] Protect onboarding routes with RequireAuth middleware
- [x] Supabase Auth integration (uses auth.users table)
- [ ] Session refresh logic when JWT expires
- [ ] Logout flow with cookie cleanup
- [ ] User profile page

### Database Schema & Migrations
- [x] Create `needs` table with status-based lifecycle
- [x] Create `need_categories` lookup table
- [x] Create `need_category_assignments` junction table
- [x] Create `need_documents` table for file uploads
- [x] Atlas configuration to only manage christjesus schema
- [x] Migration plan validated (35 statements, no drops)
- [ ] Run `just migrate` to apply schema to database
- [ ] Seed initial category data (Housing, Food, Medical, etc.)
- [ ] Create `donor_preferences` table
- [ ] Create `locations` table - City/state data for filtering

### Data Layer (Repository & Types)
- [ ] Define `pkg/types/need.go` - Need, NeedCategory, NeedDocument structs
- [ ] Define DTOs: NewNeed, UpdateNeed, NeedFilter
- [ ] Create `internal/store/need.go` repository:
  - [ ] CreateNeed(ctx, userID) - Initialize draft need
  - [ ] GetNeedByID(ctx, id) - Fetch single need
  - [ ] GetNeedsByUserID(ctx, userID) - User's needs dashboard
  - [ ] UpdateNeed(ctx, id, updates) - Update any need fields
  - [ ] UpdateNeedStep(ctx, id, step, completedSteps) - Progress tracking
  - [ ] SetNeedCategories(ctx, needID, categoryIDs) - Replace all categories
  - [ ] SubmitNeed(ctx, id) - Validate required fields, set status=submitted
  - [ ] UploadDocument(ctx, needID, doc) - Create document record
- [ ] Create `internal/store/category.go` repository:
  - [ ] GetAllCategories(ctx) - For selection UI
  - [ ] GetCategoryBySlug(ctx, slug)
  - [ ] SeedCategories(ctx) - Insert initial data

### Onboarding Implementation
- [x] Create onboarding path selection page (`/onboarding`)
- [x] Three-card layout for Need/Donor/Sponsor selection
- [x] Template structure for 8-step need flow (welcome → confirmation)
- [ ] Wire POST handlers to persist data to needs table:
  - [ ] Location handler → Update city/state/zip_code, set current_step
  - [ ] Categories handler → Insert/delete need_category_assignments
  - [ ] Details handler → Update title/amount_needed_cents/short_description
  - [ ] Story handler → Update story field
  - [ ] Documents handler → Upload to Supabase Storage, create need_documents
  - [ ] Review handler → Validate all required fields, set status=submitted
- [ ] Add form validation with error display
- [ ] Implement draft loading - pre-fill forms from existing draft
- [ ] File upload handling for documents step (Supabase Storage integration)
- [ ] Complete donor preference flow (2 steps)
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
- [x] Supabase PostgreSQL database connection (port 6543)
- [x] Supabase Auth with JWT/JWK verification
- [ ] Supabase Storage bucket for document uploads
- [ ] Configure Storage signed URLs for secure uploads
- [ ] Set up email service for notifications (Supabase Functions or SES?)
- [ ] Production deployment configuration
- [ ] Environment variable management for production

### Pending Decisions (Requires Project Owner Discussion)
- [ ] **Multi-tenancy:** Can one user have multiple roles (e.g., both donor AND have a need)?
- [ ] **Search/Filter/Recommendations:** SSR vs client-side trade-offs, HTMX implementation strategy for need discovery
- [ ] **Need Verification:** Workflow and criteria for verifying needs (gold/silver/bronze levels)
- [ ] **Notification System:** Email triggers (donations received, nearby needs posted, etc.) and delivery mechanism
