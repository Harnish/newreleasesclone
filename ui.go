package main

import "html/template"

var htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Release Tracker</title>
<style>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background: #0f172a;
    color: #e2e8f0;
    min-height: 100vh;
}

/* ---- Auth page ---- */
.auth-wrap {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
    padding: 1.5rem;
}
.auth-card {
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 10px;
    padding: 2rem;
    width: 100%;
    max-width: 380px;
}
.auth-title {
    font-size: 1.25rem;
    font-weight: 700;
    color: #60a5fa;
    margin-bottom: 0.25rem;
}
.auth-subtitle { color: #64748b; font-size: 0.82rem; margin-bottom: 1.5rem; }
.auth-tabs {
    display: flex;
    gap: 0;
    background: #0f172a;
    border-radius: 6px;
    padding: 3px;
    margin-bottom: 1.25rem;
}
.auth-tab {
    flex: 1;
    background: none;
    border: none;
    border-radius: 4px;
    color: #64748b;
    cursor: pointer;
    font-size: 0.82rem;
    font-weight: 500;
    padding: 0.45rem 0;
    transition: background 0.15s, color 0.15s;
}
.auth-tab.active { background: #1e293b; color: #e2e8f0; }
.auth-form { display: none; }
.auth-form.active { display: block; }
.auth-error {
    background: #3b0000;
    border: 1px solid #7f1d1d;
    color: #f87171;
    border-radius: 5px;
    font-size: 0.8rem;
    padding: 0.6rem 0.75rem;
    margin-bottom: 1rem;
    display: none;
}

/* ---- App header ---- */
.app-header {
    background: #1e293b;
    padding: 0.75rem 1.5rem;
    border-bottom: 1px solid #334155;
    display: flex;
    align-items: center;
    gap: 0.75rem;
}
.app-title { font-size: 1.1rem; font-weight: 700; color: #60a5fa; flex: 1; }
.user-info { display: flex; align-items: center; gap: 0.75rem; }
.username { color: #94a3b8; font-size: 0.82rem; }

/* ---- Toast ---- */
.toast {
    position: fixed;
    top: 1rem;
    right: 1rem;
    padding: 0.65rem 1rem;
    border-radius: 6px;
    font-size: 0.8rem;
    font-weight: 500;
    z-index: 200;
    opacity: 0;
    transform: translateY(-6px);
    transition: opacity 0.2s, transform 0.2s;
    pointer-events: none;
    max-width: 300px;
}
.toast.show { opacity: 1; transform: translateY(0); }
.toast.ok  { background: #052e16; color: #4ade80; border: 1px solid #14532d; }
.toast.err { background: #3b0000; color: #f87171; border: 1px solid #7f1d1d; }
.toast.inf { background: #172554; color: #93c5fd; border: 1px solid #1e3a8a; }

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

/* ---- Cards ---- */
.card {
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 8px;
    margin-bottom: 0.875rem;
    overflow: hidden;
}

/* ---- Badges ---- */
.badge {
    font-size: 0.67rem;
    font-weight: 600;
    padding: 0.18rem 0.45rem;
    border-radius: 3px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    background: #334155;
    color: #94a3b8;
}
.badge.github { background: #1e3a5f; color: #60a5fa; }
.badge.gitlab { background: #2e1f50; color: #c084fc; }
.badge.npm    { background: #103a26; color: #4ade80; }
.badge.pypi   { background: #3b2000; color: #fb923c; }
.badge.docker { background: #0c2a42; color: #38bdf8; }

/* ---- Releases accordion ---- */
.proj-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.875rem 1.1rem;
    cursor: pointer;
    transition: background 0.15s;
}
.proj-header:hover { background: #263548; }
.proj-header-left { display: flex; align-items: center; gap: 0.6rem; }
.proj-name { font-weight: 600; font-size: 0.95rem; }
.proj-right { display: flex; align-items: center; gap: 0.75rem; }
.ver-count { color: #64748b; font-size: 0.78rem; }
.chevron { color: #64748b; font-size: 0.7rem; transition: transform 0.2s; display: inline-block; }
.chevron.open { transform: rotate(180deg); }

.ver-list { display: none; border-top: 1px solid #334155; }
.ver-list.open { display: block; }

.ver-row {
    padding: 0.75rem 1.1rem;
    border-bottom: 1px solid #1a2535;
    cursor: pointer;
    transition: background 0.15s;
}
.ver-row:last-child { border-bottom: none; }
.ver-row:hover { background: #162032; }
.ver-top { display: flex; justify-content: space-between; align-items: baseline; }
.ver-tag { font-weight: 600; color: #60a5fa; font-size: 0.875rem; }
.ver-date { color: #64748b; font-size: 0.75rem; }
.ver-notes {
    display: none;
    margin-top: 0.5rem;
    padding: 0.6rem 0.75rem;
    background: #0f172a;
    border-left: 2px solid #3b82f6;
    border-radius: 0 4px 4px 0;
    font-size: 0.78rem;
    color: #94a3b8;
    line-height: 1.55;
    white-space: pre-wrap;
    word-break: break-word;
}
.ver-notes.open { display: block; }
.ver-link {
    display: inline-block;
    margin-top: 0.4rem;
    color: #60a5fa;
    font-size: 0.75rem;
    text-decoration: none;
}
.ver-link:hover { text-decoration: underline; }

/* ---- Projects tab ---- */
.proj-card {
    padding: 0.9rem 1.1rem;
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
}
.proj-card-info { flex: 1; min-width: 0; }
.proj-card-name { font-weight: 600; font-size: 0.95rem; margin-bottom: 0.25rem; }
.proj-card-meta { display: flex; gap: 0.5rem; align-items: center; flex-wrap: wrap; margin-bottom: 0.2rem; }
.proj-card-sub { color: #64748b; font-size: 0.75rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 400px; }
.proj-card-actions { display: flex; gap: 0.4rem; flex-shrink: 0; }

/* ---- Buttons ---- */
.btn {
    padding: 0.45rem 0.85rem;
    border: none;
    border-radius: 5px;
    cursor: pointer;
    font-size: 0.78rem;
    font-weight: 500;
    transition: opacity 0.15s, background 0.15s;
    white-space: nowrap;
}
.btn:disabled { opacity: 0.45; cursor: not-allowed; }
.btn-primary { background: #3b82f6; color: #fff; }
.btn-primary:hover:not(:disabled) { background: #2563eb; }
.btn-green { background: #059669; color: #fff; }
.btn-green:hover:not(:disabled) { background: #047857; }
.btn-red { background: #dc2626; color: #fff; }
.btn-red:hover:not(:disabled) { background: #b91c1c; }
.btn-outline { background: transparent; color: #94a3b8; border: 1px solid #334155; }
.btn-outline:hover:not(:disabled) { background: #1e293b; color: #e2e8f0; }
.btn-ghost { background: transparent; color: #64748b; border: none; font-size: 0.75rem; }
.btn-ghost:hover:not(:disabled) { color: #e2e8f0; }

/* ---- Form ---- */
.form-group { margin-bottom: 0.9rem; }
.form-group label { display: block; font-size: 0.8rem; color: #94a3b8; margin-bottom: 0.35rem; }
.form-group input, .form-group select {
    width: 100%;
    padding: 0.55rem 0.75rem;
    background: #0f172a;
    border: 1px solid #334155;
    border-radius: 5px;
    color: #e2e8f0;
    font-size: 0.875rem;
}
.form-group input:focus, .form-group select:focus { outline: none; border-color: #3b82f6; }
.form-group select option { background: #0f172a; }

.add-form-card { padding: 1.25rem; max-width: 480px; }
.add-form-title { font-size: 0.95rem; font-weight: 600; margin-bottom: 1rem; }

/* ---- Misc ---- */
.empty { text-align: center; padding: 4rem 2rem; color: #475569; }
.empty h3 { font-size: 0.95rem; margin-bottom: 0.4rem; }
.empty p { font-size: 0.82rem; }
.loading { text-align: center; padding: 3rem; color: #475569; font-size: 0.82rem; }
</style>
</head>
<body>

<!-- ===================== AUTH PAGE ===================== -->
<div id="auth-page">
<div class="auth-wrap">
<div class="auth-card">
    <div class="auth-title">&#x1F680; Release Tracker</div>
    <div class="auth-subtitle">Track software releases across GitHub, NPM, PyPI and more.</div>

    <div class="auth-tabs">
        <button class="auth-tab active" onclick="switchAuthTab('login', this)">Sign In</button>
        <button class="auth-tab" onclick="switchAuthTab('register', this)">Create Account</button>
    </div>

    <div id="auth-error" class="auth-error"></div>

    <form id="login-form" class="auth-form active" onsubmit="doLogin(event)">
        <div class="form-group">
            <label>Username</label>
            <input type="text" name="username" required autocomplete="username">
        </div>
        <div class="form-group">
            <label>Password</label>
            <input type="password" name="password" required autocomplete="current-password">
        </div>
        <button type="submit" class="btn btn-primary" style="width:100%" id="login-btn">Sign In</button>
    </form>

    <form id="register-form" class="auth-form" onsubmit="doRegister(event)">
        <div class="form-group">
            <label>Username <span style="color:#475569">(min 3 chars)</span></label>
            <input type="text" name="username" required minlength="3" autocomplete="username">
        </div>
        <div class="form-group">
            <label>Password <span style="color:#475569">(min 8 chars)</span></label>
            <input type="password" name="password" required minlength="8" autocomplete="new-password">
        </div>
        <button type="submit" class="btn btn-primary" style="width:100%" id="register-btn">Create Account</button>
    </form>
</div>
</div>
</div>

<!-- ===================== MAIN APP ===================== -->
<div id="app-page" style="display:none">

<header class="app-header">
    <span>&#x1F680;</span>
    <span class="app-title">Release Tracker</span>
    <div class="user-info">
        <span class="username" id="username-display"></span>
        <button class="btn btn-ghost" onclick="doLogout()">Sign out</button>
    </div>
</header>

<div id="toast" class="toast"></div>

<nav class="tabs">
    <button class="tab-btn active" data-tab="releases">Releases</button>
    <button class="tab-btn" data-tab="projects">Projects</button>
    <button class="tab-btn" data-tab="add">+ Add Project</button>
</nav>

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

</div><!-- /app-page -->

<script>
// ---- Utilities ----
function esc(s) {
    var d = document.createElement('div');
    d.textContent = s == null ? '' : String(s);
    return d.innerHTML;
}

var _toastTimer = null;
function toast(msg, type) {
    var el = document.getElementById('toast');
    if (!el) return;
    el.textContent = msg;
    el.className = 'toast show ' + (type || 'ok');
    clearTimeout(_toastTimer);
    _toastTimer = setTimeout(function() { el.classList.remove('show'); }, 3500);
}

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

// ---- Auth page ----
function switchAuthTab(tab, btn) {
    document.querySelectorAll('.auth-tab').forEach(function(b) { b.classList.remove('active'); });
    document.querySelectorAll('.auth-form').forEach(function(f) { f.classList.remove('active'); });
    btn.classList.add('active');
    document.getElementById(tab + '-form').classList.add('active');
    document.getElementById('auth-error').style.display = 'none';
}

function showAuthError(msg) {
    var el = document.getElementById('auth-error');
    el.textContent = msg;
    el.style.display = 'block';
}

function doLogin(e) {
    e.preventDefault();
    var btn = document.getElementById('login-btn');
    var data = {};
    new FormData(e.target).forEach(function(v, k) { data[k] = v; });
    btn.disabled = true; btn.textContent = 'Signing in...';
    fetch('/api/login', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(data)
    }).then(function(r) {
        if (!r.ok) return r.text().then(function(t) { throw new Error(t || 'Login failed'); });
        return r.json();
    }).then(function(user) {
        currentUser = user;
        showApp();
    }).catch(function(err) {
        showAuthError(err.message);
    }).finally(function() {
        btn.disabled = false; btn.textContent = 'Sign In';
    });
}

function doRegister(e) {
    e.preventDefault();
    var btn = document.getElementById('register-btn');
    var data = {};
    new FormData(e.target).forEach(function(v, k) { data[k] = v; });
    btn.disabled = true; btn.textContent = 'Creating account...';
    fetch('/api/register', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(data)
    }).then(function(r) {
        if (!r.ok) return r.text().then(function(t) { throw new Error(t || 'Registration failed'); });
        return r.json();
    }).then(function(user) {
        currentUser = user;
        showApp();
    }).catch(function(err) {
        showAuthError(err.message);
    }).finally(function() {
        btn.disabled = false; btn.textContent = 'Create Account';
    });
}

function doLogout() {
    fetch('/api/logout', { method: 'POST' }).finally(function() {
        currentUser = null;
        document.getElementById('app-page').style.display = 'none';
        document.getElementById('auth-page').style.display = '';
        document.getElementById('auth-error').style.display = 'none';
    });
}

// ---- App init ----
var currentUser = null;

function showApp() {
    document.getElementById('auth-page').style.display = 'none';
    document.getElementById('app-page').style.display = '';
    document.getElementById('username-display').textContent = currentUser.username;
    fetch('/api/refresh-check').catch(function() {});
    loadReleases();
    loadProjects();
}

function init() {
    fetch('/api/me')
        .then(function(r) {
            if (!r.ok) throw new Error('not authenticated');
            return r.json();
        })
        .then(function(user) {
            currentUser = user;
            showApp();
        })
        .catch(function() {
            document.getElementById('auth-page').style.display = '';
            document.getElementById('app-page').style.display = 'none';
        });
}

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

// ---- Releases tab ----
function buildReleasesHTML(releases, projects) {
    if (!releases || releases.length === 0) {
        return '<div class="empty"><h3>No releases yet</h3><p>Add projects to start tracking.</p></div>';
    }
    var projMap = {};
    (projects || []).forEach(function(p) { projMap[p.id] = p; });

    var order = [], groups = {};
    releases.forEach(function(r) {
        var pid = r.project_id || '';
        if (!groups[pid]) { groups[pid] = []; order.push(pid); }
        groups[pid].push(r);
    });

    return order.map(function(pid) {
        var proj = projMap[pid];
        var pname = proj ? proj.name : 'Unknown';
        var platform = proj ? proj.platform : '';
        var rels = groups[pid];

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
                    '<span class="ver-count">' + rels.length + ' versions</span>' +
                    '<span class="chevron">&#x25BC;</span>' +
                '</div>' +
            '</div>' +
            '<div class="ver-list">' + rows + '</div>' +
        '</div>';
    }).join('');
}

function toggleGroup(header) {
    var list = header.nextElementSibling;
    var ch = header.querySelector('.chevron');
    var open = list.classList.toggle('open');
    ch.classList.toggle('open', open);
}

function toggleNotes(row) {
    var n = row.querySelector('.ver-notes');
    if (n) n.classList.toggle('open');
}

function loadReleases() {
    document.getElementById('releases-root').innerHTML = '<div class="loading">Loading...</div>';
    Promise.all([
        fetch('/api/releases').then(function(r) { return r.json(); }),
        fetch('/api/projects').then(function(r) { return r.json(); })
    ]).then(function(res) {
        document.getElementById('releases-root').innerHTML = buildReleasesHTML(res[0], res[1]);
    }).catch(function(err) {
        document.getElementById('releases-root').innerHTML = '<div class="empty"><h3>Failed to load</h3></div>';
        console.error('loadReleases:', err);
    });
}

// ---- Projects tab ----
function buildProjectsHTML(projects) {
    if (!projects || projects.length === 0) {
        return '<div class="empty"><h3>No projects tracked</h3><p>Use "+ Add Project" to get started.</p></div>';
    }
    return projects.map(function(p) {
        var sub = p.last_refresh ? 'Refreshed ' + fmtAge(p.last_refresh) : 'Never refreshed';
        return '<div class="card proj-card">' +
            '<div class="proj-card-info">' +
                '<div class="proj-card-name">' + esc(p.name) + '</div>' +
                '<div class="proj-card-meta">' +
                    '<span class="badge ' + esc(p.platform) + '">' + esc(p.platform) + '</span>' +
                    '<span style="color:#64748b;font-size:0.75rem">' + sub + '</span>' +
                '</div>' +
                '<div class="proj-card-sub">' + esc(p.repo_url) + '</div>' +
            '</div>' +
            '<div class="proj-card-actions">' +
                '<a href="' + esc(p.repo_url) + '" target="_blank" class="btn btn-outline">View</a>' +
                '<button class="btn btn-green" data-id="' + esc(p.id) + '" onclick="doRefresh(this)">Refresh</button>' +
                '<button class="btn btn-red"   data-id="' + esc(p.id) + '" data-name="' + esc(p.name) + '" onclick="doDelete(this)">Delete</button>' +
            '</div>' +
        '</div>';
    }).join('');
}

function loadProjects() {
    document.getElementById('projects-root').innerHTML = '<div class="loading">Loading...</div>';
    fetch('/api/projects')
        .then(function(r) { return r.json(); })
        .then(function(projects) {
            document.getElementById('projects-root').innerHTML = buildProjectsHTML(projects);
        })
        .catch(function(err) {
            document.getElementById('projects-root').innerHTML = '<div class="empty"><h3>Failed to load</h3></div>';
            console.error('loadProjects:', err);
        });
}

function doRefresh(btn) {
    var id = btn.dataset.id;
    btn.disabled = true;
    var orig = btn.textContent;
    btn.textContent = 'Refreshing...';
    fetch('/api/refresh?id=' + encodeURIComponent(id), { method: 'POST' })
        .then(function(r) {
            if (r.ok) {
                toast('Refresh queued — data updates shortly.', 'inf');
                setTimeout(function() { loadProjects(); loadReleases(); }, 2000);
            } else {
                toast('Refresh failed.', 'err');
                btn.disabled = false; btn.textContent = orig;
            }
        })
        .catch(function(err) {
            toast('Error: ' + err, 'err');
            btn.disabled = false; btn.textContent = orig;
        });
}

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
                loadProjects();
                loadReleases();
            } else {
                toast('Delete failed.', 'err');
                btn.disabled = false; btn.textContent = 'Delete';
            }
        })
        .catch(function(err) {
            toast('Error: ' + err, 'err');
            btn.disabled = false; btn.textContent = 'Delete';
        });
}

// ---- Platform autofill ----
var platformConfig = {
    github: {
        label: 'Repository',
        placeholder: 'owner/repo  (e.g., kubernetes/kubernetes)',
        hint: 'Short owner/repo or paste a full GitHub URL'
    },
    gitlab: {
        label: 'Repository',
        placeholder: 'owner/repo  (e.g., gitlab-org/gitlab)',
        hint: 'Short owner/repo or paste a full GitLab URL'
    },
    npm: {
        label: 'Package name',
        placeholder: 'e.g., react  or  @scope/package',
        hint: 'Exact package name from npmjs.com'
    },
    pypi: {
        label: 'Package name',
        placeholder: 'e.g., requests',
        hint: 'Exact package name from pypi.org'
    },
    docker: {
        label: 'Image name',
        placeholder: 'e.g., nginx  or  username/image',
        hint: 'Docker Hub image name (official images: just the name)'
    },
    other: {
        label: 'URL',
        placeholder: 'Paste any supported project URL',
        hint: 'Supports GitHub, GitLab, npmjs.com, pypi.org, hub.docker.com'
    }
};

function onPlatformChange() {
    var platform = document.getElementById('add-platform').value;
    var cfg = platformConfig[platform] || {};
    document.getElementById('add-repo-label').textContent = cfg.label || 'Repository URL';
    document.getElementById('add-repo-url').placeholder = cfg.placeholder || '';
    var hintEl = document.getElementById('add-repo-hint');
    hintEl.textContent = cfg.hint || '';
    hintEl.style.color = '#475569';
    document.getElementById('add-repo-url').value = '';
    document.getElementById('add-name').value = '';
}

function detectPlatform(url) {
    if (/github\.com\//.test(url))           return 'github';
    if (/gitlab\.com\//.test(url))           return 'gitlab';
    if (/npmjs\.com\/package\//.test(url))   return 'npm';
    if (/pypi\.org\/project\//.test(url))    return 'pypi';
    if (/hub\.docker\.com\/r\//.test(url))   return 'docker';
    return null;
}

function extractNameFromURL(platform, url) {
    switch (platform) {
        case 'github':
        case 'gitlab': {
            var path = url
                .replace(/^https?:\/\/(github|gitlab)\.com\//, '')
                .replace(/\.git$/, '').replace(/\/$/, '');
            var parts = path.split('/').filter(Boolean);
            return parts.length >= 2 ? parts[1] : (parts[0] || '');
        }
        case 'npm':
            return url.replace(/^.*npmjs\.com\/package\//, '').split('/')[0];
        case 'pypi':
            return url.replace(/^.*pypi\.org\/project\//, '').split('/')[0];
        case 'docker':
            return url.replace(/^.*hub\.docker\.com\/r\//, '').replace(/\/.*$/, '');
    }
    return '';
}

function autoFillName() {
    var platform = document.getElementById('add-platform').value;
    var val = document.getElementById('add-repo-url').value.trim();
    var nameInput = document.getElementById('add-name');
    var hintEl = document.getElementById('add-repo-hint');

    if (platform === 'other') {
        var detected = detectPlatform(val);
        if (detected) {
            hintEl.textContent = 'Detected: ' + detected;
            hintEl.style.color = '#4ade80';
            nameInput.value = extractNameFromURL(detected, val);
        } else if (val) {
            hintEl.textContent = 'URL not recognized — supported: GitHub, GitLab, npmjs.com, pypi.org, hub.docker.com';
            hintEl.style.color = '#f87171';
            nameInput.value = '';
        } else {
            hintEl.textContent = platformConfig.other.hint;
            hintEl.style.color = '#475569';
            nameInput.value = '';
        }
        return;
    }

    var name = '';
    if (platform === 'github' || platform === 'gitlab') {
        var path = val
            .replace(/^https?:\/\/(github|gitlab)\.com\//, '')
            .replace(/\.git$/, '').replace(/\/$/, '');
        var parts = path.split('/').filter(Boolean);
        name = parts.length >= 2 ? parts[1] : (parts[0] || '');
    } else {
        name = val;
    }
    nameInput.value = name;
}

function expandRepoURL(platform, input) {
    input = input.trim().replace(/\.git$/, '').replace(/\/$/, '');
    switch (platform) {
        case 'github':
            return input.startsWith('http') ? input : 'https://github.com/' + input;
        case 'gitlab':
            return input.startsWith('http') ? input : 'https://gitlab.com/' + input;
        case 'npm':
            return 'https://www.npmjs.com/package/' + input;
        case 'pypi':
            return 'https://pypi.org/project/' + input;
        case 'docker':
            var ref = input.indexOf('/') !== -1 ? input : 'library/' + input;
            return 'https://hub.docker.com/r/' + ref;
    }
    return input;
}

// ---- Add project ----
document.getElementById('add-form').addEventListener('submit', function(e) {
    e.preventDefault();
    var btn = document.getElementById('add-btn');
    var data = {};
    new FormData(e.target).forEach(function(v, k) { data[k] = v; });

    if (data.platform === 'other') {
        var detected = detectPlatform(data.repo_url);
        if (!detected) {
            toast('URL not recognized. Supported: GitHub, GitLab, npmjs.com, pypi.org, hub.docker.com', 'err');
            return;
        }
        data.platform = detected;
        // For npm/pypi the fetcher uses project.Name — extract it from the URL
        if (detected === 'npm' || detected === 'pypi') {
            data.name = extractNameFromURL(detected, data.repo_url);
        }
        // repo_url is already a full URL — normalize trailing slash / .git only
        data.repo_url = data.repo_url.trim().replace(/\.git$/, '').replace(/\/$/, '');
    } else {
        data.repo_url = expandRepoURL(data.platform, data.repo_url);
    }

    btn.disabled = true; btn.textContent = 'Adding...';
    fetch('/api/projects', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(data)
    }).then(function(r) {
        if (!r.ok) return r.text().then(function(t) { throw new Error(t || 'Server error'); });
        e.target.reset();
        toast('Project added — fetching releases in background.');
        document.querySelector('[data-tab="projects"]').click();
    }).catch(function(err) {
        toast('Error: ' + err, 'err');
    }).finally(function() {
        btn.disabled = false; btn.textContent = 'Add Project';
    });
});

// ---- Boot ----
onPlatformChange();
init();
</script>
</body>
</html>
`

var tmpl = template.Must(template.New("").Parse(htmlTemplate))
