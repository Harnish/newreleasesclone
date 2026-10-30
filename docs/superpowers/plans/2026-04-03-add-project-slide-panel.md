# Add Project Slide-over Panel Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the Add Project form into a slide-over panel on the right of the Releases page, removing the nav tab bar entirely.

**Architecture:** All changes are in `ui.go` (the embedded HTML/CSS/JS string). The releases content area becomes a flex row: releases list on the left (flex: 1), slide panel on the right (max-width transition for open/close animation). No backend changes.

**Tech Stack:** Vanilla JS, CSS transitions, Go embedded template string

---

## Files

- Modify: `ui.go` — only file changed; all CSS, HTML, and JS live here as a string constant

---

### Task 1: CSS — Remove tab styles, add panel styles

**Files:**
- Modify: `ui.go`

- [ ] **Step 1: Remove the tab/panel CSS block**

In `ui.go`, find and delete the entire `/* ---- Tabs ---- */` block (lines roughly 112–134):

```css
/* ---- Tabs ---- */
.tabs {
    background: #1e293b;
    border-bottom: 1px solid #334155;
    display: flex;
    padding: 0 1.5rem;
}
.tab-btn {
    background: none;
    border: none;
    border-bottom: 2px solid transparent;
    color: #94a3b8;
    cursor: pointer;
    font-size: 0.875rem;
    padding: 0.8rem 1.1rem;
    transition: color 0.15s, border-color 0.15s;
}
.tab-btn:hover { color: #e2e8f0; }
.tab-btn.active { color: #60a5fa; border-bottom-color: #60a5fa; }

.content { max-width: 960px; margin: 0 auto; padding: 1.5rem; }
.panel { display: none; }
.panel.active { display: block; }
```

Replace with (keep `.content`, drop tab/panel rules, add new panel styles):

```css
.content { max-width: 960px; margin: 0 auto; padding: 1.5rem; }

/* ---- Releases header row ---- */
.releases-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 1rem;
}
.releases-title { font-size: 0.9rem; font-weight: 600; color: #94a3b8; }

/* ---- Releases area (list + slide panel) ---- */
.releases-area {
    display: flex;
    align-items: flex-start;
    gap: 0;
}
#releases-root { flex: 1; min-width: 0; }

/* ---- Slide-over add panel ---- */
.add-panel {
    max-width: 0;
    overflow: hidden;
    transition: max-width 0.25s ease;
    flex-shrink: 0;
}
.add-panel.open { max-width: 300px; }
.add-panel-inner {
    width: 280px;
    background: #1a2840;
    border-left: 1px solid #2d4a6e;
    padding: 1.1rem 1rem;
    min-height: 100%;
}
.add-panel-title {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 0.9rem;
    font-weight: 700;
    margin-bottom: 1.1rem;
}
```

- [ ] **Step 2: Also remove `.add-form-card` and `.add-form-title` from the Misc section**

Find and delete these two lines (they styled the old standalone add card):

```css
.add-form-card { padding: 1.25rem; max-width: 480px; }
.add-form-title { font-size: 0.95rem; font-weight: 600; margin-bottom: 1rem; }
```

- [ ] **Step 3: Verify the file still compiles**

```bash
go build -o /tmp/nr-check . && echo OK
```

Expected: `OK`

---

### Task 2: HTML — Remove nav bar, restructure content area

**Files:**
- Modify: `ui.go`

- [ ] **Step 1: Remove the entire nav bar**

Find and delete:

```html
<nav class="tabs">
    <button class="tab-btn active" data-tab="releases">Releases</button>
    <button class="tab-btn" data-tab="projects">Projects</button>
    <button class="tab-btn" data-tab="add">+ Add Project</button>
</nav>
```

- [ ] **Step 2: Replace the content div**

Find:

```html
<div class="content">
    <div id="panel-releases" class="panel active">
        <div id="releases-root"><div class="loading">Loading...</div></div>
    </div>
    <div id="panel-projects" class="panel">
        <div id="projects-root"><div class="loading">Loading...</div></div>
    </div>
    <div id="panel-add" class="panel">
        <div class="card add-form-card">
            <div class="add-form-title">Add New Project</div>
            <form id="add-form">
                <div class="form-group">
                    <label>Platform</label>
                    <select name="platform" id="add-platform" required onchange="onPlatformChange()">
                        <option value="github">GitHub</option>
                        <option value="gitlab">GitLab</option>
                        <option value="npm">NPM</option>
                        <option value="pypi">PyPI</option>
                        <option value="docker">Docker Hub</option>
                        <option value="other">Other / Custom URL</option>
                    </select>
                </div>
                <div class="form-group">
                    <label id="add-repo-label">Repository</label>
                    <input type="text" name="repo_url" id="add-repo-url" required placeholder="owner/repo" oninput="autoFillName()">
                    <span id="add-repo-hint" style="display:block;font-size:0.74rem;color:#475569;margin-top:0.3rem"></span>
                </div>
                <div class="form-group">
                    <label>Display name</label>
                    <input type="text" name="name" id="add-name" required placeholder="e.g., kubernetes">
                </div>
                <button type="submit" class="btn btn-primary" id="add-btn">Add Project</button>
            </form>
        </div>
    </div>
</div>
```

Replace with:

```html
<div class="content">
    <div class="releases-header">
        <span class="releases-title">Projects</span>
        <button class="btn btn-primary" onclick="toggleAddPanel()">+ Add Project</button>
    </div>
    <div class="releases-area">
        <div id="releases-root"><div class="loading">Loading...</div></div>
        <div id="add-panel" class="add-panel">
            <div class="add-panel-inner">
                <div class="add-panel-title">
                    Add Project
                    <button class="btn btn-ghost" onclick="closeAddPanel()" style="font-size:1rem;padding:0.2rem 0.4rem">&#x2715;</button>
                </div>
                <form id="add-form">
                    <div class="form-group">
                        <label>Platform</label>
                        <select name="platform" id="add-platform" required onchange="onPlatformChange()">
                            <option value="github">GitHub</option>
                            <option value="gitlab">GitLab</option>
                            <option value="npm">NPM</option>
                            <option value="pypi">PyPI</option>
                            <option value="docker">Docker Hub</option>
                            <option value="other">Other / Custom URL</option>
                        </select>
                    </div>
                    <div class="form-group">
                        <label id="add-repo-label">Repository</label>
                        <input type="text" name="repo_url" id="add-repo-url" required placeholder="owner/repo" oninput="autoFillName()">
                        <span id="add-repo-hint" style="display:block;font-size:0.74rem;color:#475569;margin-top:0.3rem"></span>
                    </div>
                    <div class="form-group">
                        <label>Display name</label>
                        <input type="text" name="name" id="add-name" required placeholder="e.g., kubernetes">
                    </div>
                    <button type="submit" class="btn btn-primary" id="add-btn" style="width:100%">Add Project</button>
                </form>
            </div>
        </div>
    </div>
</div>
```

- [ ] **Step 3: Verify the file compiles**

```bash
go build -o /tmp/nr-check . && echo OK
```

Expected: `OK`

---

### Task 3: JS — Remove tab logic, add panel toggle functions, fix submit handler

**Files:**
- Modify: `ui.go`

- [ ] **Step 1: Remove the tabs event listener block**

Find and delete the entire block:

```js
// ---- Tabs ----
document.querySelectorAll('.tab-btn').forEach(function(btn) {
    btn.addEventListener('click', function() {
        document.querySelectorAll('.tab-btn').forEach(function(b) { b.classList.remove('active'); });
        document.querySelectorAll('.panel').forEach(function(p) { p.classList.remove('active'); });
        btn.classList.add('active');
        document.getElementById('panel-' + btn.dataset.tab).classList.add('active');
        if (btn.dataset.tab === 'releases') loadReleases();
        if (btn.dataset.tab === 'projects') loadProjects();
    });
});
```

- [ ] **Step 2: Remove `loadProjects()` call from `showApp()`**

Find:

```js
function showApp() {
    document.getElementById('auth-page').style.display = 'none';
    document.getElementById('app-page').style.display = '';
    document.getElementById('username-display').textContent = currentUser.username;
    fetch('/api/refresh-check').catch(function() {});
    initNotifications();
    loadReleases();
    loadProjects();
}
```

Replace with:

```js
function showApp() {
    document.getElementById('auth-page').style.display = 'none';
    document.getElementById('app-page').style.display = '';
    document.getElementById('username-display').textContent = currentUser.username;
    fetch('/api/refresh-check').catch(function() {});
    initNotifications();
    loadReleases();
}
```

- [ ] **Step 3: Add panel toggle functions**

After the `// ---- App init ----` section (after the `init` function), add:

```js
// ---- Add panel ----
function toggleAddPanel() {
    var panel = document.getElementById('add-panel');
    panel.classList.toggle('open');
}

function closeAddPanel() {
    document.getElementById('add-panel').classList.remove('open');
}

document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') closeAddPanel();
});
```

- [ ] **Step 4: Fix the add-form submit handler success branch**

Find inside the add-form submit handler's `.then` success block:

```js
        e.target.reset();
        toast('Project added — fetching releases in background.');
        document.querySelector('[data-tab="projects"]').click();
```

Replace those three lines with:

```js
        e.target.reset();
        onPlatformChange();
        toast('Project added — fetching releases in background.');
        closeAddPanel();
        loadReleases();
```

(`onPlatformChange()` resets the label and hint text after the form reset.)

- [ ] **Step 5: Verify the file compiles**

```bash
go build -o /tmp/nr-check . && echo OK
```

Expected: `OK`

---

### Task 4: Run tests and manual verify

**Files:** none

- [ ] **Step 1: Run the test suite**

```bash
go test -v ./...
```

Expected: all tests pass (the tests cover HTTP handlers, not UI — all should still pass since no backend changed)

- [ ] **Step 2: Start the server**

```bash
go run . &
```

- [ ] **Step 3: Manual smoke test in browser at http://localhost:8080**

Verify:
1. Nav tab bar is gone
2. Releases page loads with "Projects" title and "+ Add Project" button
3. Clicking "+ Add Project" opens the panel (slides in from right, list narrows)
4. Clicking "+ Add Project" again closes the panel
5. Clicking ✕ inside the panel closes it
6. Pressing Escape closes the panel
7. Platform select changes update the label and hint
8. Typing in the repo field auto-fills the display name
9. Submitting the form with a valid project adds it, closes the panel, reloads releases, shows toast
10. Kill the server: `kill %1`

---

### Task 5: Commit

- [ ] **Step 1: Stage and commit**

```bash
git add ui.go
git commit -m "feat: move Add Project form to slide-over panel on Releases page"
```

- [ ] **Step 2: Update TODO.md**

Mark the first item done:

```markdown
- [x] **Add Project side panel** — move the Add Project form to a slide-in panel on the right side of the Releases page (eliminate the Projects tab)
```

```bash
git add TODO.md
git commit -m "chore: mark Add Project side panel as done"
```
