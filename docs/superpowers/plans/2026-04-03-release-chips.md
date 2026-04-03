# Release Chips Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace stacked per-release rows in each project card with a compact inline chip line showing the 5 most recent releases, with a `+N more ▾` toggle to expand the full list.

**Architecture:** All changes are confined to `ui.go` (the embedded HTML/CSS/JS string constant). No backend or schema changes required. The chip line is always visible; the full stacked list is hidden behind a `+N more ▾` button. `fmtAge` is updated to produce human-readable labels matching the desired format.

**Tech Stack:** Go (ui.go string constant), vanilla JS, CSS

---

### Task 1: Update `fmtAge` for readable labels

**Files:**
- Modify: `ui.go` — `fmtAge` function (~line 495)

The current `fmtAge` produces `1h ago`, `2d ago`, and falls back to `toLocaleDateString()` for >30 days. Update it to produce `1 hr ago`, `2 hrs ago`, `1 day ago`, `3 days ago`, `1 wk ago`, `3 wks ago`, `2 mths ago`.

- [ ] **Step 1: Replace `fmtAge` in `ui.go`**

Find this block (around line 495):
```js
function fmtAge(iso) {
    if (!iso) return '';
    var diff = Date.now() - new Date(iso).getTime();
    var m = Math.floor(diff / 60000);
    var h = Math.floor(diff / 3600000);
    var d = Math.floor(diff / 86400000);
    if (m < 2)  return 'just now';
    if (m < 60) return m + 'm ago';
    if (h < 24) return h + 'h ago';
    if (d < 30) return d + 'd ago';
    return new Date(iso).toLocaleDateString();
}
```

Replace with:
```js
function fmtAge(iso) {
    if (!iso) return '';
    var diff = Date.now() - new Date(iso).getTime();
    var m = Math.floor(diff / 60000);
    var h = Math.floor(diff / 3600000);
    var d = Math.floor(diff / 86400000);
    var w = Math.floor(d / 7);
    var mo = Math.floor(d / 30);
    if (m < 2)   return 'just now';
    if (m < 60)  return m + ' min' + (m === 1 ? '' : 's') + ' ago';
    if (h < 24)  return h + ' hr' + (h === 1 ? '' : 's') + ' ago';
    if (d < 7)   return d + ' day' + (d === 1 ? '' : 's') + ' ago';
    if (d < 30)  return w + ' wk' + (w === 1 ? '' : 's') + ' ago';
    return mo + ' mth' + (mo === 1 ? '' : 's') + ' ago';
}
```

- [ ] **Step 2: Build to verify no syntax errors**

```bash
go build -o newreleases .
```
Expected: exits 0, no errors.

- [ ] **Step 3: Commit**

```bash
git add ui.go
git commit -m "feat: update fmtAge for human-readable labels (hrs/days/wks/mths)"
```

---

### Task 2: Add CSS for chip line and expanded list

**Files:**
- Modify: `ui.go` — CSS block, after `.ver-list` rules (~line 263)

- [ ] **Step 1: Replace the existing `ver-list` CSS block and add new chip/more-list rules**

Find:
```css
.ver-list { display: none; border-top: 1px solid #334155; }
.ver-list.open { display: block; }
```

Replace with:
```css
.ver-chips {
    padding: 0.55rem 1.1rem;
    font-size: 0.8rem;
    color: #94a3b8;
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 0 0.15rem;
    border-top: 1px solid #334155;
}
.ver-chip-more {
    background: none;
    border: none;
    color: #60a5fa;
    font-size: 0.8rem;
    cursor: pointer;
    padding: 0;
    margin-left: 0.35rem;
}
.ver-chip-more:hover { text-decoration: underline; }
.ver-more-list { display: none; border-top: 1px solid #334155; }
.ver-more-list.open { display: block; }
```

- [ ] **Step 2: Remove the now-unused `.chevron` CSS rules**

Find:
```css
.chevron { color: #64748b; font-size: 0.7rem; transition: transform 0.2s; display: inline-block; }
.chevron.open { transform: rotate(180deg); }
```

Delete those two lines entirely.

- [ ] **Step 3: Build**

```bash
go build -o newreleases .
```
Expected: exits 0.

- [ ] **Step 4: Commit**

```bash
git add ui.go
git commit -m "feat: add CSS for release chip line and expandable more-list"
```

---

### Task 3: Add `toggleMore` JS function and remove `toggleGroup`

**Files:**
- Modify: `ui.go` — JS functions block (~line 682)

- [ ] **Step 1: Replace `toggleGroup` with `toggleMore`**

Find:
```js
function toggleGroup(header) {
    var list = header.nextElementSibling;
    var ch = header.querySelector('.chevron');
    var open = list.classList.toggle('open');
    ch.classList.toggle('open', open);
}
```

Replace with:
```js
function toggleMore(btn) {
    var list = btn.closest('.ver-chips').nextElementSibling;
    var open = list.classList.toggle('open');
    btn.textContent = open ? '\u25B4 less' : '+' + btn.dataset.count + ' more \u25BE';
}
```

- [ ] **Step 2: Build**

```bash
go build -o newreleases .
```
Expected: exits 0.

- [ ] **Step 3: Commit**

```bash
git add ui.go
git commit -m "feat: add toggleMore JS, remove toggleGroup"
```

---

### Task 4: Update `buildReleasesHTML` to render chips

**Files:**
- Modify: `ui.go` — `buildReleasesHTML` function (~line 636)

This is the main render change. Replace the `rows`/`ver-list` block with a chip line + optional `ver-more-list`.

- [ ] **Step 1: Replace the render block inside `buildReleasesHTML`**

Find this entire section (from `var rows = rels.map` through the closing `'</div>';`):
```js
        var rows = rels.map(function(r) {
            var notes = r.release_notes || r.description || '';
            return '<div class="ver-row" onclick="toggleNotes(this)">' +
                '<div class="ver-top">' +
                    '<span class="ver-tag">' + esc(r.version) + '</span>' +
                    '<span class="ver-date">' + fmtAge(r.published_at) + '</span>' +
                '</div>' +
                (notes
                    ? '<div class="ver-notes">' + esc(notes) +
                      '<br><a class="ver-link" href="' + esc(r.url) + '" target="_blank" onclick="event.stopPropagation()">View on ' + esc(platform || 'source') + ' &#x2197;</a></div>'
                    : '') +
            '</div>';
        }).join('');

        return '<div class="card">' +
            '<div class="proj-header" onclick="toggleGroup(this)">' +
                '<div class="proj-header-left">' +
                    '<span class="proj-name">' + esc(pname) + '</span>' +
                    (platform ? '<span class="badge ' + esc(platform) + '">' + esc(platform) + '</span>' : '') +
                '</div>' +
                '<div class="proj-right">' +
                    '<button class="btn btn-ghost proj-ctrl" title="Refresh" ' +
                        'data-id="' + esc(pid) + '" ' +
                        'onclick="event.stopPropagation();doRefresh(this)">&#x21BB;</button>' +
                    '<button class="btn btn-ghost proj-ctrl" title="Settings" ' +
                        'data-id="' + esc(pid) + '" data-name="' + esc(pname) + '" ' +
                        'data-push="' + (proj && proj.push_enabled ? '1' : '0') + '" ' +
                        'onclick="event.stopPropagation();openSettingsPanel(this)">&#x2699;</button>' +
                    '<button class="btn btn-ghost proj-ctrl" title="Delete" style="color:#f87171" ' +
                        'data-id="' + esc(pid) + '" data-name="' + esc(pname) + '" ' +
                        'onclick="event.stopPropagation();doDelete(this)">&#x2715;</button>' +
                    '<span class="ver-count">' + rels.length + ' versions</span>' +
                    '<span class="chevron">&#x25BC;</span>' +
                '</div>' +
            '</div>' +
            '<div class="ver-list">' + rows + '</div>' +
        '</div>';
```

Replace with:
```js
        var visible = rels.slice(0, 5);
        var hidden  = rels.slice(5);

        var chips = visible.map(function(r, i) {
            return fmtAge(r.published_at) + ': ' + esc(r.version) +
                (i < visible.length - 1 || hidden.length > 0 ? ',&nbsp; ' : '');
        }).join('');

        var moreBtn = hidden.length > 0
            ? '<button class="ver-chip-more" data-count="' + hidden.length + '" ' +
              'onclick="toggleMore(this)">+' + hidden.length + ' more \u25BE</button>'
            : '';

        var allRows = rels.map(function(r) {
            var notes = r.release_notes || r.description || '';
            return '<div class="ver-row" onclick="toggleNotes(this)">' +
                '<div class="ver-top">' +
                    '<span class="ver-tag">' + esc(r.version) + '</span>' +
                    '<span class="ver-date">' + fmtAge(r.published_at) + '</span>' +
                '</div>' +
                (notes
                    ? '<div class="ver-notes">' + esc(notes) +
                      '<br><a class="ver-link" href="' + esc(r.url) + '" target="_blank" onclick="event.stopPropagation()">View on ' + esc(platform || 'source') + ' &#x2197;</a></div>'
                    : '') +
            '</div>';
        }).join('');

        return '<div class="card">' +
            '<div class="proj-header">' +
                '<div class="proj-header-left">' +
                    '<span class="proj-name">' + esc(pname) + '</span>' +
                    (platform ? '<span class="badge ' + esc(platform) + '">' + esc(platform) + '</span>' : '') +
                '</div>' +
                '<div class="proj-right">' +
                    '<button class="btn btn-ghost proj-ctrl" title="Refresh" ' +
                        'data-id="' + esc(pid) + '" ' +
                        'onclick="doRefresh(this)">&#x21BB;</button>' +
                    '<button class="btn btn-ghost proj-ctrl" title="Settings" ' +
                        'data-id="' + esc(pid) + '" data-name="' + esc(pname) + '" ' +
                        'data-push="' + (proj && proj.push_enabled ? '1' : '0') + '" ' +
                        'onclick="openSettingsPanel(this)">&#x2699;</button>' +
                    '<button class="btn btn-ghost proj-ctrl" title="Delete" style="color:#f87171" ' +
                        'data-id="' + esc(pid) + '" data-name="' + esc(pname) + '" ' +
                        'onclick="doDelete(this)">&#x2715;</button>' +
                    '<span class="ver-count">' + rels.length + ' version' + (rels.length === 1 ? '' : 's') + '</span>' +
                '</div>' +
            '</div>' +
            '<div class="ver-chips">' + chips + moreBtn + '</div>' +
            '<div class="ver-more-list">' + allRows + '</div>' +
        '</div>';
```

- [ ] **Step 2: Build**

```bash
go build -o newreleases .
```
Expected: exits 0.

- [ ] **Step 3: Run tests**

```bash
go test -v ./...
```
Expected: all pass (no frontend tests; backend store/handler tests should be unaffected).

- [ ] **Step 4: Commit**

```bash
git add ui.go
git commit -m "feat: inline release chips with +N more expand on project cards"
```

---

### Task 5: Manual verification

- [ ] **Step 1: Start the server**

```bash
./newreleases
```

- [ ] **Step 2: Log in and check a project with >5 releases**

Open `http://localhost:8080`. Log in. Verify:
- The chip line appears: `1 hr ago: v1.4.0,  10 hrs ago: v1.3.2, ...`
- Ages use the new format (`hrs`, `days`, `wks`, `mths`)
- A `+N more ▾` button appears when there are >5 releases
- Clicking it expands the full stacked list below
- Clicking `▴ less` collapses it again
- The card header no longer collapses anything on click
- Refresh (↻), Settings (⚙), Delete (✕) buttons still work

- [ ] **Step 3: Check a project with ≤5 releases**

Verify: chip line shows all releases, no `+N more` button appears.

- [ ] **Step 4: Update TODO.md**

In `TODO.md`, move:
```
- [ ] **Limit releases per project** — show only the 5 most recent releases per project card, with a "more" expand link, list the releases on the same line
```
to the Done section:
```
- [x] **Limit releases per project** — inline chip line shows 5 most recent (age: version); +N more expands full list
```

Then commit:
```bash
git add TODO.md
git commit -m "chore: mark release chips feature complete in TODO"
```
