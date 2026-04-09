# Pagination Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add client-side pagination to the main releases page, with a user-persisted page size (5/10/20 projects per page).

**Architecture:** All project/release data is fetched once from existing endpoints and stored in module-level JS variables. A `renderPage()` function slices the data for the current page and re-renders the DOM. Page size is persisted in the DB (`users.page_size`) and cached in `localStorage`. A new `SetPageSize` store method and an extension to `handleAccountSettings` wire up the backend.

**Tech Stack:** Go (SQLite via `modernc.org/sqlite`), vanilla JS embedded in `ui.go` as a string constant.

> **Security note:** All DOM updates follow the existing codebase pattern — user-provided content is always passed through `esc()` before being included in HTML strings. This is the established XSS mitigation throughout the project.

---

## File Map

| File | Change |
|------|--------|
| `models.go` | Add `PageSize int` field to `User` struct |
| `store.go` | v8 to v9 migration; update `GetUserByID` scan; add `SetPageSize` method |
| `handlers.go` | Extend `handleAccountSettings` to read/write `page_size` |
| `ui.go` | Add page-size `<select>` to header; add `renderPage()`, `buildPaginationHTML()`; update `loadReleases()`, `doDelete()` |
| `main_test.go` | Tests for `SetPageSize`, `GetUserByID` page_size default, account-settings handler with page_size |

---

## Task 1: Add `PageSize` to User model and DB schema

**Files:**
- Modify: `models.go`
- Modify: `store.go` (migrate function and GetUserByID)
- Modify: `main_test.go`

- [ ] **Step 1.1: Write failing test — default page_size is 10**

Add to `main_test.go`:

```go
func TestUserPageSizeDefault(t *testing.T) {
    s := newTestStore(t)
    userID, _ := newTestAuth(t, s)

    user, err := s.GetUserByID(userID)
    if err != nil {
        t.Fatalf("GetUserByID: %v", err)
    }
    if user.PageSize != 10 {
        t.Errorf("expected default page_size 10, got %d", user.PageSize)
    }
}
```

- [ ] **Step 1.2: Run test to verify it fails**

```bash
go test -v -run TestUserPageSizeDefault ./...
```

Expected: compile error — `User` has no field `PageSize`.

- [ ] **Step 1.3: Add `PageSize` to `User` struct in `models.go`**

In `models.go`, update `User`:

```go
type User struct {
    ID            string `json:"id"`
    Username      string `json:"username"`
    Email         string `json:"email"`
    EmailVerified bool   `json:"email_verified"`
    RSSToken      string `json:"rss_token"`
    PageSize      int    `json:"page_size"`
}
```

- [ ] **Step 1.4: Add v9 migration in `store.go`**

After the `version < 8` block and before the final `return nil`, add:

```go
    if version < 9 {
        if _, err := db.Exec(`ALTER TABLE users ADD COLUMN page_size INTEGER NOT NULL DEFAULT 10`); err != nil {
            return fmt.Errorf("failed to migrate to v9 (page_size): %w", err)
        }
        if _, err := db.Exec("PRAGMA user_version = 9"); err != nil {
            return fmt.Errorf("failed to set schema version: %w", err)
        }
    }
```

- [ ] **Step 1.5: Update `GetUserByID` to scan `page_size`**

In `store.go`, replace:

```go
func (s *Store) GetUserByID(userID string) (*User, error) {
    var u User
    var emailVerified int
    err := s.db.QueryRow(
        "SELECT id, username, email, email_verified, COALESCE(rss_token,'') FROM users WHERE id = ?", userID,
    ).Scan(&u.ID, &u.Username, &u.Email, &emailVerified, &u.RSSToken)
    if err != nil {
        return nil, err
    }
    u.EmailVerified = emailVerified == 1
    return &u, nil
}
```

With:

```go
func (s *Store) GetUserByID(userID string) (*User, error) {
    var u User
    var emailVerified int
    err := s.db.QueryRow(
        "SELECT id, username, email, email_verified, COALESCE(rss_token,''), page_size FROM users WHERE id = ?", userID,
    ).Scan(&u.ID, &u.Username, &u.Email, &emailVerified, &u.RSSToken, &u.PageSize)
    if err != nil {
        return nil, err
    }
    u.EmailVerified = emailVerified == 1
    return &u, nil
}
```

- [ ] **Step 1.6: Run test to verify it passes**

```bash
go test -v -run TestUserPageSizeDefault ./...
```

Expected: PASS

- [ ] **Step 1.7: Run full test suite**

```bash
go test -v ./...
```

Expected: all tests pass.

- [ ] **Step 1.8: Commit**

```bash
git add models.go store.go main_test.go
git commit -m "feat: add page_size column to users (schema v9)"
```

---

## Task 2: Add `SetPageSize` store method

**Files:**
- Modify: `store.go`
- Modify: `main_test.go`

- [ ] **Step 2.1: Write failing tests**

Add to `main_test.go`:

```go
func TestSetPageSize(t *testing.T) {
    s := newTestStore(t)
    userID, _ := newTestAuth(t, s)

    // valid sizes
    for _, size := range []int{5, 10, 20} {
        if err := s.SetPageSize(userID, size); err != nil {
            t.Fatalf("SetPageSize(%d): %v", size, err)
        }
        user, err := s.GetUserByID(userID)
        if err != nil {
            t.Fatalf("GetUserByID: %v", err)
        }
        if user.PageSize != size {
            t.Errorf("expected page_size %d, got %d", size, user.PageSize)
        }
    }

    // invalid size rejected
    if err := s.SetPageSize(userID, 7); err == nil {
        t.Error("expected error for invalid page_size 7, got nil")
    }
}
```

- [ ] **Step 2.2: Run test to verify it fails**

```bash
go test -v -run TestSetPageSize ./...
```

Expected: compile error — `Store` has no method `SetPageSize`.

- [ ] **Step 2.3: Add `SetPageSize` to `store.go`**

After `SetEmailDigest`, add:

```go
func (s *Store) SetPageSize(userID string, size int) error {
    if size != 5 && size != 10 && size != 20 {
        return fmt.Errorf("invalid page_size %d: must be 5, 10, or 20", size)
    }
    _, err := s.db.Exec("UPDATE users SET page_size = ? WHERE id = ?", size, userID)
    return err
}
```

- [ ] **Step 2.4: Run test to verify it passes**

```bash
go test -v -run TestSetPageSize ./...
```

Expected: PASS

- [ ] **Step 2.5: Run full test suite**

```bash
go test -v ./...
```

Expected: all tests pass.

- [ ] **Step 2.6: Commit**

```bash
git add store.go main_test.go
git commit -m "feat: add SetPageSize store method"
```

---

## Task 3: Extend `handleAccountSettings` for `page_size`

**Files:**
- Modify: `handlers.go`
- Modify: `main_test.go`

- [ ] **Step 3.1: Write failing tests**

Add to `main_test.go`:

```go
func TestHandleAccountSettingsPageSizeGET(t *testing.T) {
    originalStore := store
    defer func() { store = originalStore }()
    store = newTestStore(t)

    _, cookie := newTestAuth(t, store)

    req, _ := http.NewRequest("GET", "/api/account-settings", nil)
    req.AddCookie(cookie)
    w := httptest.NewRecorder()
    requireAuth(handleAccountSettings).ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
    }
    var resp map[string]interface{}
    if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
        t.Fatalf("failed to decode response: %v", err)
    }
    ps, ok := resp["page_size"]
    if !ok {
        t.Fatal("expected page_size in response")
    }
    if int(ps.(float64)) != 10 {
        t.Errorf("expected default page_size 10, got %v", ps)
    }
}

func TestHandleAccountSettingsPageSizePOST(t *testing.T) {
    originalStore := store
    defer func() { store = originalStore }()
    store = newTestStore(t)

    userID, cookie := newTestAuth(t, store)

    body := `{"page_size":5}`
    req, _ := http.NewRequest("POST", "/api/account-settings", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    req.AddCookie(cookie)
    w := httptest.NewRecorder()
    requireAuth(handleAccountSettings).ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
    }
    user, err := store.GetUserByID(userID)
    if err != nil {
        t.Fatalf("GetUserByID: %v", err)
    }
    if user.PageSize != 5 {
        t.Errorf("expected page_size 5, got %d", user.PageSize)
    }
}

func TestHandleAccountSettingsPageSizeInvalid(t *testing.T) {
    originalStore := store
    defer func() { store = originalStore }()
    store = newTestStore(t)

    _, cookie := newTestAuth(t, store)

    body := `{"page_size":7}`
    req, _ := http.NewRequest("POST", "/api/account-settings", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    req.AddCookie(cookie)
    w := httptest.NewRecorder()
    requireAuth(handleAccountSettings).ServeHTTP(w, req)

    if w.Code != http.StatusBadRequest {
        t.Errorf("expected 400 for invalid page_size, got %d", w.Code)
    }
}
```

- [ ] **Step 3.2: Run tests to verify they fail**

```bash
go test -v -run "TestHandleAccountSettingsPageSize" ./...
```

Expected: `TestHandleAccountSettingsPageSizeGET` fails (no `page_size` in response).

- [ ] **Step 3.3: Update `handleAccountSettings` in `handlers.go`**

Replace the existing `handleAccountSettings` function:

```go
func handleAccountSettings(w http.ResponseWriter, r *http.Request, userID string) {
    w.Header().Set("Content-Type", "application/json")
    switch r.Method {
    case http.MethodGet:
        enabled, err := store.GetUserDigestEnabled(userID)
        if err != nil {
            http.Error(w, "user not found", http.StatusNotFound)
            return
        }
        user, err := store.GetUserByID(userID)
        if err != nil {
            http.Error(w, "user not found", http.StatusNotFound)
            return
        }
        json.NewEncoder(w).Encode(map[string]interface{}{
            "email_digest": enabled,
            "page_size":    user.PageSize,
        })

    case http.MethodPost:
        var req struct {
            EmailDigest *bool `json:"email_digest"`
            PageSize    *int  `json:"page_size"`
        }
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        if req.EmailDigest != nil {
            if err := store.SetEmailDigest(userID, *req.EmailDigest); err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
            }
        }
        if req.PageSize != nil {
            if err := store.SetPageSize(userID, *req.PageSize); err != nil {
                http.Error(w, err.Error(), http.StatusBadRequest)
                return
            }
        }
        json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
}
```

- [ ] **Step 3.4: Run new tests to verify they pass**

```bash
go test -v -run "TestHandleAccountSettingsPageSize" ./...
```

Expected: all three PASS

- [ ] **Step 3.5: Run full test suite**

```bash
go test -v ./...
```

Expected: all tests pass.

- [ ] **Step 3.6: Commit**

```bash
git add handlers.go main_test.go
git commit -m "feat: handle page_size in account-settings API"
```

---

## Task 4: Frontend — state, selector, pagination, updated loadReleases and doDelete

**Files:**
- Modify: `ui.go`

All changes are to HTML/CSS/JS embedded as a string constant in `ui.go`. Content is always passed through the existing `esc()` helper before being included in HTML strings.

- [ ] **Step 4.1: Add module-level state variables**

In the JS section of `ui.go`, after `var currentUser = null;`, add:

```js
var allProjects = [];
var allReleases = [];
var currentPage = 1;
var pageSize = parseInt(localStorage.getItem('page_size'), 10) || 10;
```

- [ ] **Step 4.2: Add CSS for pagination controls**

In the CSS section of `ui.go`, after `.releases-title { ... }`, add:

```css
.page-size-select {
    background: #1a2840;
    color: #e2e8f0;
    border: 1px solid #2d4a6e;
    border-radius: 4px;
    padding: 0.25rem 0.4rem;
    font-size: 0.82rem;
    cursor: pointer;
}
.pagination {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 1rem;
    margin-top: 1.2rem;
    font-size: 0.85rem;
    color: #94a3b8;
}
.pagination button {
    background: #1a2840;
    color: #e2e8f0;
    border: 1px solid #2d4a6e;
    border-radius: 4px;
    padding: 0.25rem 0.7rem;
    cursor: pointer;
    font-size: 0.82rem;
}
.pagination button:disabled {
    opacity: 0.35;
    cursor: default;
}
```

- [ ] **Step 4.3: Add page-size selector to the releases header**

Find this HTML block in `ui.go`:

```html
    <div class="releases-header">
        <span class="releases-title">Projects</span>
        <button class="btn btn-primary" onclick="toggleAddPanel()">+ Add Project</button>
    </div>
```

Replace with:

```html
    <div class="releases-header">
        <span class="releases-title">Projects</span>
        <div style="display:flex;align-items:center;gap:0.6rem">
            <label style="font-size:0.8rem;color:#94a3b8" for="page-size-select">Per page:</label>
            <select id="page-size-select" class="page-size-select" onchange="onPageSizeChange(this.value)">
                <option value="5">5</option>
                <option value="10" selected>10</option>
                <option value="20">20</option>
            </select>
            <button class="btn btn-primary" onclick="toggleAddPanel()">+ Add Project</button>
        </div>
    </div>
```

- [ ] **Step 4.4: Add `buildPaginationHTML()` after `buildReleasesHTML`**

```js
function buildPaginationHTML() {
    var total = allProjects.length;
    var totalPages = Math.ceil(total / pageSize);
    if (totalPages <= 1) return '';
    return '<div class="pagination">' +
        '<button onclick="goToPage(' + (currentPage - 1) + ')"' +
            (currentPage <= 1 ? ' disabled' : '') + '>\u2190 Prev</button>' +
        '<span>Page ' + currentPage + ' of ' + totalPages + '</span>' +
        '<button onclick="goToPage(' + (currentPage + 1) + ')"' +
            (currentPage >= totalPages ? ' disabled' : '') + '>Next \u2192</button>' +
    '</div>';
}
```

- [ ] **Step 4.5: Add `renderPage()` after `buildPaginationHTML`**

```js
function renderPage() {
    var start = (currentPage - 1) * pageSize;
    var pageProjects = allProjects.slice(start, start + pageSize);
    var pageIDs = {};
    pageProjects.forEach(function(p) { pageIDs[p.id] = true; });
    var pageReleases = allReleases.filter(function(r) { return pageIDs[r.project_id]; });
    // All content in buildReleasesHTML and buildPaginationHTML is passed through esc().
    document.getElementById('releases-root').innerHTML =
        buildReleasesHTML(pageReleases, pageProjects) + buildPaginationHTML();
    var sel = document.getElementById('page-size-select');
    if (sel) sel.value = String(pageSize);
}
```

- [ ] **Step 4.6: Add `goToPage()` after `renderPage`**

```js
function goToPage(n) {
    var totalPages = Math.ceil(allProjects.length / pageSize) || 1;
    if (n < 1 || n > totalPages) return;
    currentPage = n;
    renderPage();
}
```

- [ ] **Step 4.7: Add `onPageSizeChange()` after `goToPage`**

```js
function onPageSizeChange(val) {
    var n = parseInt(val, 10);
    if (n !== 5 && n !== 10 && n !== 20) return;
    pageSize = n;
    currentPage = 1;
    localStorage.setItem('page_size', String(n));
    fetch('/api/account-settings', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({page_size: n})
    }).catch(function() {
        toast('Failed to save page size preference', 'err');
    });
    renderPage();
}
```

- [ ] **Step 4.8: Replace `loadReleases()`**

Find and replace the existing `loadReleases` function:

```js
function loadReleases() {
    document.getElementById('releases-root').innerHTML = '<div class="loading">Loading...</div>';
    Promise.all([
        fetch('/api/releases').then(function(r) { return r.json(); }),
        fetch('/api/projects').then(function(r) { return r.json(); })
    ]).then(function(res) {
        allReleases = res[0] || [];
        allProjects = res[1] || [];
        currentPage = 1;
        renderPage();
        if (_settingsRepoID) {
            var fresh = allProjects.filter(function(p) { return p.id === _settingsRepoID; })[0];
            if (fresh) {
                loadSettingsPanel(_settingsRepoID, fresh.push_enabled);
            } else {
                closeSettingsPanel();
                _settingsRepoID = null;
            }
        }
    }).catch(function(err) {
        document.getElementById('releases-root').innerHTML =
            '<div class="empty"><h3>Failed to load</h3></div>';
        console.error('loadReleases:', err);
    });
}
```

- [ ] **Step 4.9: Replace `doDelete()`**

Find and replace the existing `doDelete` function:

```js
function doDelete(btn) {
    var id   = btn.dataset.id;
    var name = btn.dataset.name || id;
    if (!window.confirm('Delete "' + name + '"? This cannot be undone.')) return;
    btn.disabled = true;
    btn.textContent = 'Deleting...';
    fetch('/api/projects?id=' + encodeURIComponent(id), { method: 'DELETE' })
        .then(function(r) {
            if (r.ok) {
                toast('Project deleted.');
                allProjects = allProjects.filter(function(p) { return p.id !== id; });
                allReleases = allReleases.filter(function(rel) { return rel.project_id !== id; });
                var totalPages = Math.ceil(allProjects.length / pageSize) || 1;
                if (currentPage > totalPages) currentPage = totalPages;
                renderPage();
            } else {
                toast('Delete failed.', 'err');
                btn.disabled = false;
                btn.textContent = '\u2715';
            }
        })
        .catch(function(err) {
            toast('Error: ' + err, 'err');
            btn.disabled = false;
            btn.textContent = '\u2715';
        });
}
```

- [ ] **Step 4.10: Sync `pageSize` from `/api/me` on login**

In the `showApp` function, find:

```js
    fetch('/api/refresh-check').catch(function() {});
    initNotifications();
    loadReleases();
```

Replace with:

```js
    fetch('/api/refresh-check').catch(function() {});
    initNotifications();
    if (currentUser.page_size && [5, 10, 20].indexOf(currentUser.page_size) !== -1) {
        pageSize = currentUser.page_size;
        localStorage.setItem('page_size', String(pageSize));
    }
    loadReleases();
```

- [ ] **Step 4.11: Build and smoke-test**

```bash
go build -o newreleases . && ./newreleases
```

Open `http://localhost:8080`. Verify:
- "Per page:" selector appears in header with options 5, 10, 20
- With 10+ projects, pages render correctly
- Prev/Next disable at boundaries
- Delete updates the list without a full page reload
- Changing page size resets to page 1
- After reload, the previously chosen page size is restored

- [ ] **Step 4.12: Commit**

```bash
git add ui.go
git commit -m "feat: client-side pagination with page size selector"
```

---

## Task 5: Update TODO.md

**Files:**
- Modify: `TODO.md`

- [ ] **Step 5.1: Mark pagination done**

In `TODO.md`, move the pagination item from `## Planned` to `## Done`, changing `[ ]` to `[x]`:

```markdown
- [x] **Pagenate releases and projects** - Allow the user to set the number of projects per page between 5,10 and 20.  Pagenate beyond that number.
```

- [ ] **Step 5.2: Commit**

```bash
git add TODO.md
git commit -m "chore: mark pagination as done in TODO"
```
