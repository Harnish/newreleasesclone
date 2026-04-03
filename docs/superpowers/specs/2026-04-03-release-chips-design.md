# Design: Inline Release Chips Per Project Card

**Date:** 2026-04-03  
**Status:** Approved

## Summary

Replace the stacked per-release rows inside each project card with a single inline "chips" line showing the 5 most recent releases. If more than 5 exist, a `+N more ▾` link expands the full stacked list below.

## Current Behavior

Each project card contains a `ver-list` div (hidden by default, toggled open via the card header chevron). Inside are stacked `ver-row` divs — one per release — each showing version tag, age, and optionally expandable release notes.

## New Behavior

### Chip line

Always-visible (no collapse). Format:

```
1 hr ago: v1.4.0,  10 hrs ago: v1.3.2,  2 days ago: v1.3.1,  3 wks ago: v1.3.0,  2 mths ago: v1.2.9  +45 more ▾
```

- Shows the 5 most recent releases as plain inline text, comma-separated
- Each entry: `<fmtAge(published_at)>: <version>`
- If `rels.length > 5`, appends a `+N more ▾` button at the end
- The chip line sits directly below the `proj-header`, no border/padding gap

### Expand behavior

Clicking `+N more ▾` toggles a `ver-more-list` div (hidden by default) containing the full stacked `ver-row` list (all releases, existing layout with notes expand). The button text toggles to `▴ less`.

### Header changes

- Remove the `toggleGroup` click handler and chevron from `proj-header` — the card header no longer collapses the chip line
- Keep the `ver-count` badge ("12 versions") in the header right side

## Components

### CSS

| Class | Purpose |
|---|---|
| `.ver-chips` | Flex-wrap container for the chip line; `font-size: 0.8rem; color: #94a3b8; padding: 0.6rem 1.1rem; flex-wrap: wrap; gap: 0.25rem 0.5rem;` |
| `.ver-chip-more` | The `+N more ▾` button; ghost style, same font size |
| `.ver-more-list` | Full stacked list; `display: none` by default; `.open` shows it |

### JS

**`buildReleasesHTML`** — updated render logic:
1. Take `rels` (all releases for project, already sorted newest-first from API)
2. Render first 5 as comma-separated `<age>: <version>` spans inside `.ver-chips`
3. If `rels.length > 5`, append a `.ver-chip-more` button with `+N more ▾`
4. Render all releases as `ver-row` divs inside `.ver-more-list` (hidden)
5. Remove `onclick="toggleGroup(this)"` and `.chevron` from `proj-header`

**`toggleMore(btn)`** — new function:
```js
function toggleMore(btn) {
    var list = btn.parentElement.nextElementSibling;
    var open = list.classList.toggle('open');
    btn.textContent = open
        ? '▴ less'
        : '+' + btn.dataset.count + ' more ▾';
}
```

## What Is NOT Changing

- `fmtAge()` — reused as-is
- `ver-row` / `ver-notes` / `toggleNotes` — reused inside `.ver-more-list`
- `ver-count` in header
- Refresh, Settings, Delete buttons in header
- API responses — no backend changes needed

## Testing

Manual: add a project with >5 releases (GitHub works well), verify chip line shows 5, `+N more` button appears, clicking expands stacked list, clicking again collapses. Projects with ≤5 releases show no `+N more` button.
