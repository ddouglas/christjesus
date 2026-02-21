# Context Blurb for Another Project (Go SSR + Tailwind)

Use the following in another project when asking an AI assistant to generate all Go templates.

## Copy/Paste Prompt

I am migrating a React/Next UI into Go SSR. Do not generate React. Generate only Go templates (`html/template`) or Templ components.

Requirements:

1. Preserve Tailwind classes exactly from my source snippets unless a class is invalid in my Tailwind version.
2. Keep design tokens from `tokens.css` untouched.
3. Build reusable partials/components first, then page templates.
4. Use backend-rendered variables and loops for all dynamic content.
5. Prefer simple, composable templates over framework-like abstractions.
6. No JavaScript unless explicitly required.

Input artifacts to use:

- `tokens.css` (theme/tokens)
- `components.html` (static snippets)
- existing route map from the current app

Output structure I want:

- `web/templates/layouts/base.tmpl`
- `web/templates/partials/header.tmpl`
- `web/templates/partials/footer.tmpl`
- `web/templates/partials/need-card.tmpl`
- `web/templates/pages/home.tmpl`
- `web/templates/pages/browse.tmpl`
- `web/templates/pages/need-detail.tmpl`

Also generate:

- Go view models matching these templates in `internal/viewmodels/*.go`
- sample handlers showing template execution in `internal/http/handlers/*.go`
- sample route setup

Rules for dynamic data binding:

- Use `{{ .Field }}` for scalar fields.
- Use `{{ range .Items }}` for lists.
- Use `{{ if .Condition }}` for conditional sections.
- Use template funcs for formatting money/percent/date.

If using Templ instead of html/template:

- Create one `templ` component per extracted section.
- Keep parameter names explicit (`need NeedViewModel`, `city string`, etc).
- Compose pages from partial components.

Delivery order:

1. Base layout + header/footer partials.
2. Card primitives + need card component.
3. Need detail page.
4. Browse page with looping cards.
5. Remaining pages.

After generation, provide:

- a file tree of all generated templates,
- one command to run Tailwind build,
- one command to run the Go server,
- and a short parity checklist against the source design.
