# Go + HTMX Web Application Scaffolding Guide

**Based on the Todoer project architecture - Updated February 2026**

## Project Architecture Overview

This is a modern Go web application using HTMX for dynamic UI interactions without JavaScript. The architecture follows Go best practices with a clean separation of concerns.

## Technology Stack

### Backend
- **Go 1.21+** - Modern Go with latest features
- **HTTP Router**: `alexedwards/flow` - Lightweight, method-based routing
- **Database**: PostgreSQL 18+ via `pgxpool` for connection pooling
- **Query Builder**: `Masterminds/squirrel` - Type-safe SQL query construction
- **Row Scanning**: `georgysavva/scany/v2` - Elegant struct scanning from pgx queries
- **Migrations**: Atlas with HCL schema-as-code (declarative, not sequential)
- **Logging**: `logrus` - Structured logging with fields
- **Config**: `kelseyhightower/envconfig` + YAML files for hybrid config management
- **CLI**: `urfave/cli/v2` - Multi-command CLI with subcommands (serve, worker, etc.)
- **ID Generation**: `go-nanoid` - Short, URL-safe unique IDs

### Frontend
- **HTMX 2.0.8** - Dynamic HTML interactions
- **CSS Framework**: Tailwind CSS
- **Templates**: Go `html/template` with `embed.FS` for binary embedding

### Infrastructure
- **Docker Compose** - Local PostgreSQL development environment
- **Task Runner**: `justfile` (modern Make alternative)

## Project Structure

```
project/
├── cmd/
│   └── appname/              # CLI entrypoint and commands
│       ├── main.go           # urfave/cli app definition
│       ├── serve.go          # HTTP server command with graceful shutdown
│       ├── worker.go         # Background worker command (optional)
│       └── config.go         # Config loading (envconfig)
├── internal/
│   ├── server/               # HTTP layer
│   │   ├── server.go         # Server initialization, router, template loading
│   │   ├── middleware.go     # Logging, auth, etc.
│   │   ├── {resource}.go     # Handler files per resource (todo.go, user.go)
│   │   └── templates/        # Embedded HTML templates
│   │       ├── layout.html   # Base layout shell
│   │       ├── components/   # Reusable UI components
│   │       │   ├── nav.html
│   │       │   ├── {resource}-item.html
│   │       │   └── modal.html
│   │       └── pages/        # Full page templates
│   │           └── home.html
│   ├── store/                # Data access layer (repositories)
│   │   ├── {resource}.go     # One file per database table/resource
│   │   └── stmt.go           # Shared SQL helpers (psql builder, etc.)
│   ├── notify/               # External integrations (optional)
│   │   └── ntfy.go
│   └── utils/                # Shared utilities
│       ├── nano.go           # ID generation wrapper
│       └── structs.go        # Reflection helpers for struct tags
├── pkg/
│   └── types/                # Public types, models, DTOs
│       ├── config.go         # Config struct with envconfig tags
│       └── {resource}.go     # Domain models with `db:"column"` tags
├── migrations/               # Atlas HCL schema definitions
│   ├── atlas.hcl             # Atlas configuration
│   └── {table}.pg.hcl        # One file per table (declarative schema)
├── bin/                      # Build output
├── docker-compose.yaml       # Local database
├── justfile                  # Task definitions
├── go.mod
├── config.yml.example        # Example configuration
└── PROJECT_STATE.md          # Living documentation (critical!)
```

## Architecture Patterns

### 1. Repository Pattern (Store Layer)

One repository per table/resource in `internal/store/`. Each repository wraps `*pgxpool.Pool` and provides type-safe methods.

**Example: `internal/store/todo.go`**

```go
package store

import (
    "context"
    "fmt"
    sq "github.com/Masterminds/squirrel"
    "github.com/georgysavva/scany/v2/pgxscan"
    "github.com/jackc/pgx/v5/pgxpool"
)

const todoTableName = "todos"

var todoTableColumns = []string{"id", "title", "description", "completed", "created_at", "updated_at"}

type TodoRepository struct {
    pool *pgxpool.Pool
}

func NewTodoRepository(pool *pgxpool.Pool) *TodoRepository {
    return &TodoRepository{pool: pool}
}

func (r *TodoRepository) Todo(ctx context.Context, id string) (*types.Todo, error) {
    query, args, err := psql().
        Select(todoTableColumns...).
        From(todoTableName).
        Where(sq.Eq{"id": id}).
        ToSql()
    if err != nil {
        return nil, fmt.Errorf("failed to generate query: %w", err)
    }

    var todo = new(types.Todo)
    err = pgxscan.Get(ctx, r.pool, todo, query, args...)
    if err != nil {
        return nil, err
    }

    return todo, nil
}

func (r *TodoRepository) Todos(ctx context.Context) ([]*types.Todo, error) {
    query, args, err := psql().
        Select(todoTableColumns...).
        From(todoTableName).
        OrderBy("created_at DESC").
        ToSql()
    if err != nil {
        return nil, fmt.Errorf("failed to generate query: %w", err)
    }

    var todos = make([]*types.Todo, 0)
    err = pgxscan.Select(ctx, r.pool, &todos, query, args...)
    if err != nil {
        return nil, err
    }

    return todos, nil
}

func (r *TodoRepository) CreateTodo(ctx context.Context, todo *types.Todo) error {
    todo.ID = generateID() // from utils/nano.go
    
    query, args, err := psql().
        Insert(todoTableName).
        Columns("id", "title", "description", "completed").
        Values(todo.ID, todo.Title, todo.Description, todo.Completed).
        ToSql()
    if err != nil {
        return fmt.Errorf("failed to generate query: %w", err)
    }

    _, err = r.pool.Exec(ctx, query, args...)
    return err
}

func (r *TodoRepository) UpdateTodo(ctx context.Context, todo *types.Todo) error {
    query, args, err := psql().
        Update(todoTableName).
        Set("title", todo.Title).
        Set("description", todo.Description).
        Set("completed", todo.Completed).
        Set("updated_at", sq.Expr("now()")).
        Where(sq.Eq{"id": todo.ID}).
        ToSql()
    if err != nil {
        return fmt.Errorf("failed to generate query: %w", err)
    }

    _, err = r.pool.Exec(ctx, query, args...)
    return err
}

func (r *TodoRepository) DeleteTodo(ctx context.Context, id string) error {
    query, args, err := psql().
        Delete(todoTableName).
        Where(sq.Eq{"id": id}).
        ToSql()
    if err != nil {
        return fmt.Errorf("failed to generate query: %w", err)
    }

    _, err = r.pool.Exec(ctx, query, args...)
    return err
}
```

**Shared SQL Helper: `internal/store/stmt.go`**

```go
package store

import (
    sq "github.com/Masterminds/squirrel"
)

func psql() sq.StatementBuilderType {
    return sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
}
```

**Key Patterns:**
- Methods return pointers to domain models from `pkg/types/`
- Use Squirrel for query building with PostgreSQL placeholder format (`$1`, `$2`)
- Use Scany for elegant struct scanning
- Handle joins explicitly by loading related entities after main query
- Return wrapped errors with context

---

### 2. Template Architecture (Component-Based)

**Embedded Templates Pattern**

```go
package server

import (
    "embed"
    "html/template"
    "io/fs"
    "strings"
)

//go:embed templates
var templateFS embed.FS

func (s *Service) loadTemplates() {
    tmpl := template.New("")
    
    err := fs.WalkDir(templateFS, "templates", func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return err
        }
        
        if d.IsDir() || !strings.HasSuffix(path, ".html") {
            return nil
        }
        
        data, err := fs.ReadFile(templateFS, path)
        if err != nil {
            return fmt.Errorf("failed to read template at %s: %w", path, err)
        }
        
        _, err = tmpl.Parse(string(data))
        if err != nil {
            return fmt.Errorf("failed to parse template at %s: %w", path, err)
        }
        
        return nil
    })
    
    if err != nil {
        s.logger.WithError(err).Fatal("failed to load templates")
    }
    
    s.templates = tmpl
}
```

**Template Naming Convention:**
- `layout.base` - HTML shell with blocks
- `component.{name}` - Reusable fragments (nav, sidebar, item cards)
- `page.{name}` - Full pages that extend layout

**Base Layout: `templates/layout.html`**

```html
{{define "layout.base"}}
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{block "title" .}}App Name{{end}}</title>
    <link rel="stylesheet" href="#css url">
    {{block "head" .}}{{end}}
</head>
<body>
    <div class="app-container">
        {{template "component.nav" .}}
        <div class="content-wrapper">
            {{block "sidebar" .}}{{end}}
            <main class="container">
                {{block "page-content" .}}{{end}}
            </main>
        </div>
    </div>
    
    <!-- Modal Container for HTMX -->
    <div id="modal-container"></div>
    
    <script src="https://cdn.jsdelivr.net/npm/htmx.org@2.0.8/dist/htmx.min.js"></script>
    {{block "scripts" .}}{{end}}
</body>
</html>
{{end}}
```

**Component Example: `templates/components/todo-item.html`**

```html
{{define "component.todo.item"}}
<div id="todo-{{.ID}}" class="todo-item">
    <article>
        <header>
            <h4>{{.Title}}</h4>
            {{if .Category}}
                <span class="badge" style="background: {{.Category.Color}}">
                    {{if .Category.Icon}}{{.Category.Icon}}{{end}} {{.Category.Name}}
                </span>
            {{end}}
        </header>
        
        {{if .Description}}
        <p>{{.Description}}</p>
        {{end}}
        
        <footer>
            <button hx-patch="/todos/{{.ID}}" 
                    hx-target="#todo-{{.ID}}"
                    hx-swap="outerHTML">
                {{if .Completed}}Undo{{else}}Complete{{end}}
            </button>
            <button hx-delete="/todos/{{.ID}}" 
                    hx-target="#todo-{{.ID}}"
                    hx-swap="delete"
                    hx-confirm="Delete this todo?">
                Delete
            </button>
        </footer>
    </article>
</div>
{{end}}
```

**Page Example: `templates/pages/home.html`**

```html
{{define "page.home"}}
{{template "layout.base" .}}

{{define "page-content"}}
<div class="home-page">
    <div class="header">
        <h1>Todos</h1>
        <button hx-get="/todos/new/modal" 
                hx-target="#modal-container">
            New Todo
        </button>
    </div>
    
    <div id="todos-list">
        {{if .Todos}}
            {{range .Todos}}
                {{template "component.todo.item" .}}
            {{end}}
        {{else}}
            {{template "component.todo.empty" .}}
        {{end}}
    </div>
</div>
{{end}}

{{end}}
```

---

### 3. Server Initialization Pattern

**Server Struct: `internal/server/server.go`**

```go
package server

import (
    "context"
    "embed"
    "fmt"
    "html/template"
    "net/http"
    "time"
    
    "github.com/alexedwards/flow"
    "github.com/sirupsen/logrus"
)

//go:embed templates
var templateFS embed.FS

type Service struct {
    port   uint
    logger *logrus.Logger
    
    // Repositories
    todos      *store.TodoRepository
    categories *store.CategoryRepository
    
    templates *template.Template
    server    *http.Server
}

const defaultTimeout = time.Second * 5

func New(
    port uint,
    logger *logrus.Logger,
    todos *store.TodoRepository,
    categories *store.CategoryRepository,
) *Service {
    mux := flow.New()
    
    s := &Service{
        port:       port,
        logger:     logger,
        todos:      todos,
        categories: categories,
        server: &http.Server{
            Addr:              fmt.Sprintf(":%d", port),
            Handler:           mux,
            ReadTimeout:       defaultTimeout,
            ReadHeaderTimeout: defaultTimeout,
            WriteTimeout:      defaultTimeout,
            MaxHeaderBytes:    512,
        },
    }
    
    s.buildRouter(mux)
    s.loadTemplates()
    
    return s
}

func (s *Service) Start() error {
    return s.server.ListenAndServe()
}

func (s *Service) Stop(ctx context.Context) error {
    return s.server.Shutdown(ctx)
}

func (s *Service) buildRouter(r *flow.Mux) {
    r.Use(s.LoggingMiddleware)
    
    // Pages
    r.HandleFunc("/", s.handleHome, http.MethodGet)
    
    // Todo CRUD
    r.HandleFunc("/todos", s.handlePostTodo, http.MethodPost)
    r.HandleFunc("/todos/new/modal", s.handleNewTodoModal, http.MethodGet)
    r.HandleFunc("/todos/:id", s.handleGetTodo, http.MethodGet)
    r.HandleFunc("/todos/:id", s.handlePutTodo, http.MethodPut)
    r.HandleFunc("/todos/:id", s.handlePatchTodo, http.MethodPatch)
    r.HandleFunc("/todos/:id", s.handleDeleteTodo, http.MethodDelete)
    r.HandleFunc("/todos/:id/modal", s.handleEditTodoModal, http.MethodGet)
}

func (s *Service) internalServerError(w http.ResponseWriter) {
    http.Error(w, "internal server error", http.StatusInternalServerError)
}
```

---

### 4. Handler Organization

Group handlers by resource in separate files. Always use typed page data structs.

**Example: `internal/server/todo.go`**

```go
package server

import (
    "net/http"
    
    "github.com/alexedwards/flow"
)

// Page Data Structs
type HomePageData struct {
    Todos []*types.Todo
}

// Create Todo
func (s *Service) handlePostTodo(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    err := r.ParseForm()
    if err != nil {
        s.logger.WithError(err).Error("failed to parse form")
        s.internalServerError(w)
        return
    }
    
    todo := &types.Todo{
        Title:       r.FormValue("title"),
        Description: r.FormValue("description"),
        Completed:   r.FormValue("completed") == "on",
    }
    
    err = s.todos.CreateTodo(ctx, todo)
    if err != nil {
        s.logger.WithError(err).Error("failed to create todo")
        s.internalServerError(w)
        return
    }
    
    // Return component for HTMX
    w.Header().Set("Content-Type", "text/html")
    err = s.templates.ExecuteTemplate(w, "component.todo.item", todo)
    if err != nil {
        s.logger.WithError(err).Error("failed to render template")
    }
}

// Toggle Todo (PATCH)
func (s *Service) handlePatchTodo(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    id := flow.Param(r.Context(), "id")
    
    todo, err := s.todos.Todo(ctx, id)
    if err != nil {
        s.logger.WithError(err).Error("failed to fetch todo")
        http.NotFound(w, r)
        return
    }
    
    // Toggle completion
    todo.Completed = !todo.Completed
    
    err = s.todos.UpdateTodo(ctx, todo)
    if err != nil {
        s.logger.WithError(err).Error("failed to update todo")
        s.internalServerError(w)
        return
    }
    
    // Return updated component
    w.Header().Set("Content-Type", "text/html")
    err = s.templates.ExecuteTemplate(w, "component.todo.item", todo)
    if err != nil {
        s.logger.WithError(err).Error("failed to render template")
    }
}

// Delete Todo
func (s *Service) handleDeleteTodo(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    id := flow.Param(r.Context(), "id")
    
    err := s.todos.DeleteTodo(ctx, id)
    if err != nil {
        s.logger.WithError(err).Error("failed to delete todo")
        s.internalServerError(w)
        return
    }
    
    w.WriteHeader(http.StatusOK)
}

// Home Page
func (s *Service) handleHome(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    todos, err := s.todos.Todos(ctx)
    if err != nil {
        s.logger.WithError(err).Error("failed to fetch todos")
        s.internalServerError(w)
        return
    }
    
    data := HomePageData{
        Todos: todos,
    }
    
    w.Header().Set("Content-Type", "text/html")
    err = s.templates.ExecuteTemplate(w, "page.home", data)
    if err != nil {
        s.logger.WithError(err).Error("failed to render template")
    }
}

// Modal for creating new todo
func (s *Service) handleNewTodoModal(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html")
    err := s.templates.ExecuteTemplate(w, "component.modal.todo-form", nil)
    if err != nil {
        s.logger.WithError(err).Error("failed to render modal")
    }
}
```

**Handler Best Practices:**
- Always set `Content-Type: text/html` for template responses
- Use `flow.Param(r.Context(), "id")` to get route parameters
- Log errors with structured fields, return generic HTTP errors to client
- Return single component templates for HTMX partial updates
- Return full page templates for standard GET requests

---

### 5. Configuration Management (Hybrid Pattern)

Support both YAML files and environment variables with override capability.

**Config Struct: `pkg/types/config.go`**

```go
package types

import "fmt"

type Config struct {
    Environment string `yaml:"environment" envconfig:"ENVIRONMENT"`
    Port        uint   `yaml:"port" envconfig:"PORT"`
    Database    DatabaseConfig `yaml:"database"`
}

type DatabaseConfig struct {
    URL      string `yaml:"url" envconfig:"DATABASE_URL"`
    Host     string `yaml:"host" envconfig:"DB_HOST"`
    Port     int    `yaml:"port" envconfig:"DB_PORT"`
    User     string `yaml:"user" envconfig:"DB_USER"`
    Password string `yaml:"password" envconfig:"DB_PASSWORD"`
    Database string `yaml:"database" envconfig:"DB_NAME"`
    SSLMode  string `yaml:"sslmode" envconfig:"DB_SSLMODE"`
}

func (d *DatabaseConfig) BuildDatabaseURL() string {
    if d.URL != "" {
        return d.URL
    }
    return fmt.Sprintf(
        "postgres://%s:%s@%s:%d/%s?sslmode=%s",
        d.User, d.Password, d.Host, d.Port, d.Database, d.SSLMode,
    )
}
```

**Config Loading: `cmd/appname/config.go`**

```go
package main

import (
    "fmt"
    "os"
    
    "github.com/kelseyhightower/envconfig"
    "gopkg.in/yaml.v3"
)

var c = new(types.Config)

func LoadConfig(configPath string) error {
    // Load YAML file first (optional)
    if configPath != "" {
        data, err := os.ReadFile(configPath)
        if err != nil {
            return fmt.Errorf("failed to read config file: %w", err)
        }
        
        err = yaml.Unmarshal(data, c)
        if err != nil {
            return fmt.Errorf("failed to parse config file: %w", err)
        }
    }
    
    // Override with environment variables
    err := envconfig.Process("APPNAME", c)
    if err != nil {
        return fmt.Errorf("failed to process env vars: %w", err)
    }
    
    // Validate
    if err := validateConfig(); err != nil {
        return err
    }
    
    // Build complex values
    if c.Database.URL == "" {
        c.Database.URL = c.Database.BuildDatabaseURL()
    }
    
    return nil
}

func validateConfig() error {
    if c.Port == 0 {
        return fmt.Errorf("port is required")
    }
    if c.Database.URL == "" && c.Database.Host == "" {
        return fmt.Errorf("database config required")
    }
    return nil
}
```

**Example config.yml:**

```yaml
environment: development
port: 8080

database:
  host: localhost
  port: 5432
  user: appname
  password: password
  database: appname
  sslmode: disable
```

---

### 6. Database Patterns

#### Atlas Migrations (HCL Schema-as-Code)

Atlas uses declarative schema definition - you define the desired state, not migration steps.

**Atlas Config: `migrations/atlas.hcl`**

```hcl
env "local" {
  src = data.hcl_schema.app.url
  url = "postgres://appname:password@localhost:5432/appname?sslmode=disable"
}

env "production" {
  src = data.hcl_schema.app.url
  url = getenv("DATABASE_URL")
}

data "hcl_schema" "app" {
  paths = fileset("*.pg.hcl")
}
```

**Table Schema: `migrations/todos.pg.hcl`**

```hcl
schema "public" {}

table "todos" {
  schema = schema.public
  
  column "id" {
    type = text
  }
  
  column "title" {
    type = text
    null = false
  }
  
  column "description" {
    type = text
    null = true
  }
  
  column "completed" {
    type    = boolean
    null    = false
    default = false
  }
  
  column "priority" {
    type    = text
    null    = false
    default = "medium"
  }
  
  column "category_id" {
    type = text
    null = true
  }
  
  column "due_date" {
    type = timestamp
    null = true
  }
  
  column "created_at" {
    type    = timestamp
    null    = false
    default = sql("now()")
  }
  
  column "updated_at" {
    type    = timestamp
    null    = false
    default = sql("now()")
  }
  
  primary_key {
    columns = [column.id]
  }
  
  index "idx_todos_category_id" {
    columns = [column.category_id]
  }
  
  index "idx_todos_due_date" {
    columns = [column.due_date]
  }
  
  foreign_key "fk_todos_category" {
    columns     = [column.category_id]
    ref_columns = [table.categories.column.id]
    on_delete   = SET_NULL
  }
}

table "categories" {
  schema = schema.public
  
  column "id" {
    type = text
  }
  
  column "name" {
    type = text
    null = false
  }
  
  column "color" {
    type    = text
    null    = false
    default = "#3b82f6"
  }
  
  column "created_at" {
    type    = timestamp
    null    = false
    default = sql("now()")
  }
  
  primary_key {
    columns = [column.id]
  }
}
```

**Atlas Commands:**
```bash
# Apply migrations
atlas schema apply --env local --auto-approve

# Preview changes (dry-run)
atlas schema apply --env local --dry-run

# Inspect current schema
atlas schema inspect --env local
```

#### Query Building with Squirrel + Scany

**Helper for struct tag extraction: `internal/utils/structs.go`**

```go
package utils

import "reflect"

var ColumnTag = "db"

func StructTagValues(input any) []string {
    targetValue := reflect.ValueOf(input)
    if targetValue.Kind() == reflect.Ptr {
        targetValue = targetValue.Elem()
    }
    
    if targetValue.Kind() != reflect.Struct {
        panic("input must be a struct or pointer to struct")
    }
    
    targetType := targetValue.Type()
    result := make([]string, 0, targetValue.NumField())
    
    for i := 0; i < targetValue.NumField(); i++ {
        if targetType.Field(i).PkgPath != "" {
            continue // unexported field
        }
        
        tagValue := targetType.Field(i).Tag.Get(ColumnTag)
        if tagValue == "" || tagValue == "-" {
            continue
        }
        
        result = append(result, tagValue)
    }
    
    return result
}
```

**Usage:**
```go
var todoTableColumns = StructTagValues(&types.Todo{})
// Returns: ["id", "title", "description", "completed", "created_at", "updated_at"]

query, args, _ := psql().
    Select(todoTableColumns...).
    From("todos").
    Where(sq.Eq{"id": id}).
    ToSql()
```

---

### 7. HTMX Integration Patterns

#### Standard CRUD Operations

**Create (Append Pattern)**
```html
<form hx-post="/todos" 
      hx-target="#todos-list" 
      hx-swap="afterbegin">
    <input type="text" name="title" required>
    <button type="submit">Add</button>
</form>
```

**Update (Swap Pattern)**
```html
<button hx-patch="/todos/{{.ID}}" 
        hx-target="#todo-{{.ID}}"
        hx-swap="outerHTML">
    Toggle
</button>
```

**Delete (Remove Pattern)**
```html
<button hx-delete="/todos/{{.ID}}" 
        hx-target="#todo-{{.ID}}"
        hx-swap="delete"
        hx-confirm="Are you sure?">
    Delete
</button>
```

#### Modal Pattern

**Modal Container in Layout:**
```html
<div id="modal-container"></div>
```

**Load Modal:**
```html
<button hx-get="/todos/new/modal" 
        hx-target="#modal-container">
    New Todo
</button>
```

**Modal Component: `templates/components/modal.html`**
```html
{{define "component.modal.todo-form"}}
<div class="modal" id="todo-modal">
    <div class="modal-content">
        <h2>New Todo</h2>
        <form hx-post="/todos" 
              hx-target="#todos-list" 
              hx-swap="afterbegin"
              hx-on::after-request="document.getElementById('todo-modal').remove()">
            <input type="text" name="title" required>
            <button type="submit">Create</button>
            <button type="button" onclick="this.closest('.modal').remove()">Cancel</button>
        </form>
    </div>
</div>
<script>
// Auto-show modal when loaded
document.getElementById('todo-modal').style.display = 'block';
</script>
{{end}}
```

#### Out-of-Band (OOB) Swaps

Update multiple parts of the page from one request:

```html
{{define "component.todo.item"}}
<div id="todo-{{.ID}}" class="todo-item">
    <!-- Main content -->
</div>

<!-- Update counter elsewhere on page -->
<span id="todo-count" hx-swap-oob="true">
    {{.TotalCount}} todos
</span>
{{end}}
```

---

### 8. CLI Command Pattern (urfave/cli)

**Main: `cmd/appname/main.go`**

```go
package main

import (
    "os"
    
    "github.com/sirupsen/logrus"
    "github.com/urfave/cli/v2"
)

func main() {
    app := &cli.App{
        Name:  "appname",
        Usage: "Description of your app",
        Flags: []cli.Flag{
            &cli.StringFlag{
                Name:    "config",
                Aliases: []string{"c"},
                Usage:   "Load configuration from `FILE`",
                EnvVars: []string{"APP_CONFIG"},
                Value:   "config.yml",
            },
        },
        Commands: []*cli.Command{
            serveCommand,
            workerCommand,
        },
    }
    
    err := app.Run(os.Args)
    if err != nil {
        logrus.WithError(err).Fatal("failed to run application")
    }
}
```

**Serve Command: `cmd/appname/serve.go`**

```go
package main

import (
    "context"
    "errors"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/sirupsen/logrus"
    "github.com/urfave/cli/v2"
)

var serveCommand = &cli.Command{
    Name:   "serve",
    Usage:  "Start the HTTP server",
    Action: serve,
}

func serve(cCtx *cli.Context) error {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()
    
    logger := logrus.New()
    logger.SetFormatter(&logrus.JSONFormatter{})
    
    // Load config
    err := LoadConfig(cCtx.String("config"))
    if err != nil {
        logger.WithError(err).Fatal("failed to load config")
    }
    
    // Connect to database
    pool, err := pgxpool.New(ctx, c.Database.URL)
    if err != nil {
        logger.WithError(err).Fatal("failed to connect to database")
    }
    defer pool.Close()
    
    err = pool.Ping(ctx)
    if err != nil {
        logger.WithError(err).Fatal("failed to ping database")
    }
    
    // Initialize repositories
    todoStore := store.NewTodoRepository(pool)
    
    // Initialize server
    srv := server.New(c.Port, logger, todoStore)
    
    // Start server in goroutine
    go func() {
        logger.WithField("port", c.Port).Info("server starting")
        err := srv.Start()
        if err != nil && !errors.Is(err, http.ErrServerClosed) {
            logger.WithError(err).Fatal("server error")
        }
    }()
    
    // Wait for interrupt signal
    <-ctx.Done()
    logger.Info("shutting down gracefully...")
    
    // Graceful shutdown with timeout
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    err = srv.Stop(shutdownCtx)
    if err != nil {
        logger.WithError(err).Error("shutdown error")
        return err
    }
    
    logger.Info("server stopped")
    return nil
}
```

---

### 9. Middleware Pattern

**Logging Middleware: `internal/server/middleware.go`**

```go
package server

import (
    "net/http"
    "time"
    
    "github.com/sirupsen/logrus"
)

type responseWriter struct {
    http.ResponseWriter
    statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.statusCode = code
    rw.ResponseWriter.WriteHeader(code)
}

func (s *Service) LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
        
        next.ServeHTTP(rw, r)
        
        s.logger.WithFields(logrus.Fields{
            "method":      r.Method,
            "path":        r.URL.Path,
            "status":      rw.statusCode,
            "duration_ms": time.Since(start).Milliseconds(),
        }).Info("request")
    })
}
```

---

### 10. Worker/Background Task Pattern

For processing background jobs, reminders, notifications, etc.

**Worker Command: `cmd/appname/worker.go`**

```go
package main

import (
    "context"
    "time"
    
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/sirupsen/logrus"
    "github.com/urfave/cli/v2"
)

var workerCommand = &cli.Command{
    Name:   "worker",
    Usage:  "Run background worker (designed for cron/scheduled execution)",
    Action: runWorker,
}

func runWorker(cCtx *cli.Context) error {
    ctx := context.Background()
    
    logger := logrus.New()
    logger.SetFormatter(&logrus.JSONFormatter{})
    
    // Load config
    err := LoadConfig(cCtx.String("config"))
    if err != nil {
        logger.WithError(err).Fatal("failed to load config")
    }
    
    // Connect to database
    pool, err := pgxpool.New(ctx, c.Database.URL)
    if err != nil {
        logger.WithError(err).Fatal("failed to connect to database")
    }
    defer pool.Close()
    
    // Initialize store
    todoStore := store.NewTodoRepository(pool)
    
    logger.Info("worker started")
    
    // Process pending tasks
    err = processPendingTasks(ctx, logger, todoStore)
    if err != nil {
        logger.WithError(err).Error("worker error")
        return err
    }
    
    logger.Info("worker completed")
    return nil
}

func processPendingTasks(ctx context.Context, logger *logrus.Logger, store *store.TodoRepository) error {
    // Find todos with reminders due
    todos, err := store.TodosWithRemindersDue(ctx, time.Now())
    if err != nil {
        return err
    }
    
    logger.WithField("count", len(todos)).Info("processing reminders")
    
    for _, todo := range todos {
        // Send notification
        logger.WithField("todo_id", todo.ID).Info("sending reminder")
        // ... notification logic ...
    }
    
    return nil
}
```

**Systemd Timer (Linux) or Cron:**
```bash
# Run every 5 minutes
*/5 * * * * /path/to/appname worker
```

---

### 11. Utility Helpers

**ID Generation: `internal/utils/nano.go`**

```go
package utils

import gonanoid "github.com/matoous/go-nanoid/v2"

func GenerateID() string {
    id, _ := gonanoid.New(21)
    return id
}
```

**Domain Models: `pkg/types/todo.go`**

```go
package types

import "time"

type Todo struct {
    ID          string     `db:"id"`
    Title       string     `db:"title"`
    Description *string    `db:"description"`
    Completed   bool       `db:"completed"`
    Priority    string     `db:"priority"`
    CategoryID  *string    `db:"category_id"`
    DueDate     *time.Time `db:"due_date"`
    CreatedAt   time.Time  `db:"created_at"`
    UpdatedAt   time.Time  `db:"updated_at"`
    
    // Joined fields (not in DB)
    Category *Category `db:"-"`
}

type Category struct {
    ID        string    `db:"id"`
    Name      string    `db:"name"`
    Color     string    `db:"color"`
    CreatedAt time.Time `db:"created_at"`
}

const (
    PriorityLow    = "low"
    PriorityMedium = "medium"
    PriorityHigh   = "high"
)
```

---

## Development Workflow

### Justfile (Task Runner)

Create a `justfile` in project root:

```just
# List available commands
default:
    @just --list

# Run database migrations
migrate:
    atlas schema apply --env local --auto-approve

# Check migration plan (dry run)
migrate-plan:
    atlas schema apply --env local --dry-run

# Start the HTTP server
serve:
    go run cmd/appname/*.go -c config.yml serve

# Start the background worker
worker:
    go run cmd/appname/*.go -c config.yml worker

# Build the binary
build:
    go build -o bin/appname cmd/appname/*.go

# Clean build artifacts
clean:
    rm -rf bin/

# Format Go code
fmt:
    go fmt ./...

# Run tests
test:
    go test ./...

# Install dependencies
deps:
    go mod download
    go mod tidy

# Start docker compose services
docker-up:
    docker compose up -d

# Stop docker compose services
docker-down:
    docker compose down

# Show database schema
db-inspect:
    atlas schema inspect --env local
```

### Docker Compose

**docker-compose.yaml:**

```yaml
services:
  db:
    image: postgres:18-alpine
    container_name: appname-db
    environment:
      POSTGRES_USER: appname
      POSTGRES_PASSWORD: password
      POSTGRES_DB: appname
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U appname"]
      interval: 10s
      timeout: 5s
      retries: 5

volumes:
  postgres_data:
```

### Go Module Setup

```bash
go mod init github.com/yourusername/appname

# Add dependencies
go get github.com/alexedwards/flow
go get github.com/jackc/pgx/v5/pgxpool
go get github.com/Masterminds/squirrel
go get github.com/georgysavva/scany/v2
go get github.com/kelseyhightower/envconfig
go get github.com/sirupsen/logrus
go get github.com/urfave/cli/v2
go get github.com/matoous/go-nanoid/v2
go get gopkg.in/yaml.v3
```

---

## Key Conventions & Best Practices

### Code Organization
1. **Internal packages only** - Use `internal/` to prevent external imports
2. **Public types in pkg** - Only stable, reusable types in `pkg/`
3. **One handler file per resource** - Keeps code navigable
4. **Table columns from struct tags** - Single source of truth via reflection

### Error Handling
1. **Log then return generic errors** - Never expose internal errors to clients
2. **Use structured logging** - `logger.WithError(err).WithField("key", val).Error()`
3. **Fatal only at startup** - Use `.Fatal()` for initialization failures only
4. **Wrap errors with context** - `fmt.Errorf("context: %w", err)`

### Database
1. **Use context everywhere** - Pass `context.Context` to all DB methods
2. **Pointers for nullable fields** - `*string`, `*time.Time` for NULL columns
3. **Separate loading for joins** - Load related entities after main query
4. **Text IDs over UUIDs** - Use nanoid for shorter, URL-friendly IDs
5. **Always use prepared statements** - Squirrel + pgx handles this automatically

### Templates
1. **Always use blocks** - Define `{{block}}` in layout for extensibility
2. **Component templates for HTMX** - Return single component for partial updates
3. **Set Content-Type explicitly** - Always `w.Header().Set("Content-Type", "text/html")`
4. **Use template functions** - `eq`, `ne` for comparisons
5. **Handle nil pointers** - Check `{{if .Field}}` before accessing pointer fields

### HTTP/HTMX
1. **REST-ish routes** - Use proper HTTP methods (GET, POST, PUT, PATCH, DELETE)
2. **Return appropriate status codes** - 200, 201, 204, 400, 404, 500
3. **HTMX targets** - Always specify `hx-target` for clarity
4. **Modal containers** - Dedicated `<div id="modal-container">` in layout
5. **OOB for multi-updates** - Use `hx-swap-oob` to update multiple page sections

### Configuration
1. **Sensible defaults** - Don't require all env vars
2. **YAML for complexity** - Use YAML for nested config, env vars for overrides
3. **Validation function** - Separate `validateConfig()` after loading
4. **Build methods on config** - Add helpers like `BuildDatabaseURL()`

---

## Project Initialization Checklist

When scaffolding a new project:

- [ ] Create directory structure (cmd, internal, pkg, migrations)
- [ ] Initialize Go module (`go mod init`)
- [ ] Add core dependencies (see Go Module Setup section)
- [ ] Create `docker-compose.yaml` with PostgreSQL
- [ ] Create `justfile` with standard targets
- [ ] Set up configuration (config.go, types/config.go, config.yml.example)
- [ ] Create Atlas migration files (`atlas.hcl` + initial table schemas)
- [ ] Set up server architecture (server.go, middleware.go)
- [ ] Create base layout template with HTMX
- [ ] Create first domain model in `pkg/types/`
- [ ] Create first repository in `internal/store/`
- [ ] Create first handler in `internal/server/`
- [ ] Create first templates (page + components)
- [ ] Add ID generation utility
- [ ] Set up CLI commands (main.go, serve.go)
- [ ] Test database connection and migrations
- [ ] Test first CRUD endpoint with HTMX
- [ ] Document in `PROJECT_STATE.md`

---

## Minimal Starter Example

Here's a minimal working example to get started:

### 1. Initialize Project

```bash
mkdir myapp && cd myapp
go mod init github.com/yourusername/myapp
mkdir -p cmd/myapp internal/{server,store,utils} pkg/types migrations
```

### 2. Add Dependencies

```bash
go get github.com/alexedwards/flow \
  github.com/jackc/pgx/v5/pgxpool \
  github.com/Masterminds/squirrel \
  github.com/georgysavva/scany/v2 \
  github.com/kelseyhightower/envconfig \
  github.com/sirupsen/logrus \
  github.com/urfave/cli/v2 \
  github.com/matoous/go-nanoid/v2 \
  gopkg.in/yaml.v3
```

### 3. Start with Core Files

Focus on these files first:
1. `cmd/myapp/main.go` - CLI entry point
2. `cmd/myapp/serve.go` - HTTP server command
3. `cmd/myapp/config.go` - Config loading
4. `pkg/types/config.go` - Config struct
5. `pkg/types/todo.go` - First domain model
6. `internal/store/stmt.go` - SQL helper
7. `internal/store/todo.go` - First repository
8. `internal/server/server.go` - Server initialization
9. `internal/server/todo.go` - First handler
10. `internal/server/templates/layout.html` - Base layout
11. `migrations/atlas.hcl` - Atlas config
12. `migrations/todos.pg.hcl` - First table
13. `docker-compose.yaml` - Database
14. `justfile` - Task definitions

Build incrementally from there!

---

## Additional Resources

### Documentation Links
- **HTMX**: https://htmx.org/docs/
- **alexedwards/flow**: https://github.com/alexedwards/flow
- **pgx**: https://github.com/jackc/pgx
- **Squirrel**: https://github.com/Masterminds/squirrel
- **Scany**: https://github.com/georgysavva/scany
- **Atlas**: https://atlasgo.io/docs
- **Logrus**: https://github.com/sirupsen/logrus
- **urfave/cli**: https://github.com/urfave/cli

### Common Patterns Not Covered
- Authentication/Authorization (sessions, JWT)
- WebSocket integration
- File uploads
- API versioning
- Rate limiting
- Caching strategies
- Multi-tenancy

These can be added incrementally as project needs grow.

---

## Living Documentation

Always maintain a `PROJECT_STATE.md` file with:
- **Tech stack** - What libraries/tools you're using
- **Project structure** - How code is organized
- **Architecture decisions** - Why you chose certain patterns
- **Current features** - What's implemented and working
- **Database schema** - Tables, columns, indexes, relationships
- **Routes/endpoints** - API surface area
- **Known issues/quirks** - Gotchas and workarounds
- **Development commands** - How to run, build, test
- **Environment variables** - Required configuration

This becomes your single source of truth and helps AI assistants understand your project context.

---

**End of Guide**

This scaffolding approach has been validated on production applications and provides a solid foundation for building maintainable Go + HTMX web applications. Start simple, iterate, and expand based on your specific needs.
