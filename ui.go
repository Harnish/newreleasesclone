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

.app-header {
    background: #1e293b;
    padding: 1rem 1.5rem;
    border-bottom: 1px solid #334155;
    display: flex;
    align-items: center;
    gap: 0.6rem;
}
.app-title { font-size: 1.15rem; font-weight: 700; color: #60a5fa; }

.toast {
    position: fixed;
    top: 1rem;
    right: 1rem;
    padding: 0.7rem 1.1rem;
    border-radius: 6px;
    font-size: 0.82rem;
    font-weight: 500;
    z-index: 200;
    opacity: 0;
    transform: translateY(-6px);
    transition: opacity 0.2s, transform 0.2s;
    pointer-events: none;
    max-width: 320px;
}
.toast.show { opacity: 1; transform: translateY(0); }
.toast.ok  { background: #052e16; color: #4ade80; border: 1px solid #14532d; }
.toast.err { background: #3b0000; color: #f87171; border: 1px solid #7f1d1d; }
.toast.inf { background: #172554; color: #93c5fd; border: 1px solid #1e3a8a; }

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
    padding: 0.875rem 1.1rem;
    transition: color 0.15s, border-color 0.15s;
}
.tab-btn:hover { color: #e2e8f0; }
.tab-btn.active { color: #60a5fa; border-bottom-color: #60a5fa; }

.content { max-width: 960px; margin: 0 auto; padding: 1.5rem; }
.panel { display: none; }
.panel.active { display: block; }

.card {
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 8px;
    margin-bottom: 0.875rem;
    overflow: hidden;
}

/* Releases accordion */
.proj-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.9rem 1.1rem;
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

/* Projects tab */
.proj-card {
    padding: 0.9rem 1.1rem;
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
}
.proj-card-info { flex: 1; min-width: 0; }
.proj-card-name { font-weight: 600; font-size: 0.95rem; margin-bottom: 0.3rem; }
.proj-card-meta { display: flex; gap: 0.5rem; align-items: center; flex-wrap: wrap; margin-bottom: 0.2rem; }
.proj-card-sub { color: #64748b; font-size: 0.75rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 400px; }
.proj-card-actions { display: flex; gap: 0.4rem; flex-shrink: 0; }

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
.btn-green  { background: #059669; color: #fff; }
.btn-green:hover:not(:disabled)  { background: #047857; }
.btn-red    { background: #dc2626; color: #fff; }
.btn-red:hover:not(:disabled)    { background: #b91c1c; }
.btn-outline { background: transparent; color: #94a3b8; border: 1px solid #334155; }
.btn-outline:hover:not(:disabled) { background: #1e293b; color: #e2e8f0; }

/* Add form */
.form-wrap { max-width: 480px; padding: 1.5rem; }
.form-title { font-size: 1rem; font-weight: 600; margin-bottom: 1.1rem; color: #e2e8f0; }
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

.empty { text-align: center; padding: 4rem 2rem; color: #475569; }
.empty h3 { font-size: 0.95rem; margin-bottom: 0.4rem; }
.empty p { font-size: 0.82rem; }
.loading { text-align: center; padding: 3rem; color: #475569; font-size: 0.82rem; }
</style>
</head>
<body>

<header class="app-header">
    <span>&#x1F680;</span>
    <span class="app-title">Release Tracker</span>
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
        <div class="card form-wrap">
            <div class="form-title">Add New Project</div>
            <form id="add-form">
                <div class="form-group">
                    <label>Project Name</label>
                    <input type="text" name="name" required placeholder="e.g., kubernetes">
                </div>
                <div class="form-group">
                    <label>Platform</label>
                    <select name="platform" required>
                        <option value="github">GitHub</option>
                        <option value="gitlab">GitLab</option>
                        <option value="npm">NPM</option>
                        <option value="pypi">PyPI</option>
                        <option value="docker">Docker Hub</option>
                    </select>
                </div>
                <div class="form-group">
                    <label>Repository URL</label>
                    <input type="url" name="repo_url" required placeholder="https://github.com/owner/repo">
                </div>
                <button type="submit" class="btn btn-primary" id="add-btn">Add Project</button>
            </form>
        </div>
    </div>
</div>

<script>
function esc(s) {
    var d = document.createElement('div');
    d.textContent = s == null ? '' : String(s);
    return d.innerHTML;
}

var _toastTimer = null;
function toast(msg, type) {
    var el = document.getElementById('toast');
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
        return '<div class="empty"><h3>No releases yet</h3><p>Add projects to start tracking releases.</p></div>';
    }

    var projMap = {};
    (projects || []).forEach(function(p) { projMap[p.id] = p; });

    // Group by project_id preserving first-seen order
    var order = [];
    var groups = {};
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
        document.getElementById('releases-root').innerHTML = '<div class="empty"><h3>Failed to load releases</h3></div>';
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
        return '<div class="card proj-card" data-id="' + esc(p.id) + '">' +
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
            document.getElementById('projects-root').innerHTML = '<div class="empty"><h3>Failed to load projects</h3></div>';
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
                toast('Refresh queued — data updates in a moment.', 'inf');
                setTimeout(function() { loadProjects(); loadReleases(); }, 2000);
            } else {
                toast('Refresh failed.', 'err');
                btn.disabled = false;
                btn.textContent = orig;
            }
        })
        .catch(function(err) {
            toast('Refresh error: ' + err, 'err');
            btn.disabled = false;
            btn.textContent = orig;
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
                btn.disabled = false;
                btn.textContent = 'Delete';
            }
        })
        .catch(function(err) {
            toast('Delete error: ' + err, 'err');
            btn.disabled = false;
            btn.textContent = 'Delete';
        });
}

// ---- Add project ----
document.getElementById('add-form').addEventListener('submit', function(e) {
    e.preventDefault();
    var btn = document.getElementById('add-btn');
    var data = {};
    new FormData(e.target).forEach(function(v, k) { data[k] = v; });
    btn.disabled = true;
    btn.textContent = 'Adding...';
    fetch('/api/projects', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
    }).then(function(r) {
        if (!r.ok) return r.text().then(function(t) { throw new Error(t || 'Server error'); });
        e.target.reset();
        toast('Project added — fetching releases in background.');
        document.querySelector('[data-tab="projects"]').click();
    }).catch(function(err) {
        toast('Error: ' + err, 'err');
    }).finally(function() {
        btn.disabled = false;
        btn.textContent = 'Add Project';
    });
});

// ---- Init ----
fetch('/api/refresh-check').catch(function() {});
loadReleases();
loadProjects();
</script>
</body>
</html>
`

var tmpl = template.Must(template.New("").Parse(htmlTemplate))
