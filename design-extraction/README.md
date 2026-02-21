# Tailwind Design Extraction (No React)

This folder contains a backend-agnostic design extraction from this Next.js app for a Go SSR proof of concept.

## Files

- `tokens.css`: full design tokens, color variables, typography variables, base + utility layers.
- `components.html`: static Tailwind component snippets (header, footer, card, need card, need detail shell).
- `go-template-context-blurb.md`: reusable prompt/context to generate full Go templates in another repo.

## Use in Go SSR (html/template)

1. Copy `tokens.css` into your Go web project (example: `web/static/css/tokens.css`).
2. Ensure your Tailwind pipeline includes that file in the build.
3. Split `components.html` into partials:
   - `web/templates/partials/header.tmpl`
   - `web/templates/partials/footer.tmpl`
   - `web/templates/partials/need-card.tmpl`
4. Bind backend data through template fields (`{{ .Need.FirstName }}`, `{{ .Need.GoalAmount }}`, etc).
5. Keep class strings unchanged first; optimize later after parity is confirmed.

## Use in Templ

Map each section of `components.html` to one `templ` component function:

- `Header(activeCity string)`
- `Footer()`
- `NeedCard(need NeedViewModel)`
- `NeedDetailPage(vm NeedDetailViewModel)`

Start with static output parity, then add conditionals and loops.

## Font note

The original app used Next font loaders. In Go SSR, include font links in your base layout, for example:

```html
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=Montserrat:wght@500;600;700&family=Poppins:wght@500;600;700&display=swap" rel="stylesheet">
```
