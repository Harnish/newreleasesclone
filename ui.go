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

/* ---- Slide-over settings panel ---- */
.settings-panel {
    max-width: 0;
    overflow: hidden;
    transition: max-width 0.25s ease;
    flex-shrink: 0;
}
.settings-panel.open { max-width: 380px; }
.settings-panel-inner {
    width: 360px;
    background: #1a2840;
    border-left: 1px solid #2d4a6e;
    padding: 1.1rem 1rem;
    min-height: 100%;
}
/* ---- Account settings panel ---- */
.account-panel {
    max-width: 0;
    overflow: hidden;
    transition: max-width 0.25s ease;
    flex-shrink: 0;
}
.account-panel.open { max-width: 300px; }
.account-panel-inner {
    width: 280px;
    background: #1a2840;
    border-left: 1px solid #2d4a6e;
    padding: 1.1rem 1rem;
    min-height: 100%;
}
.settings-section { margin-bottom: 1.25rem; }
.settings-section-title {
    font-size: 0.72rem;
    font-weight: 700;
    color: #64748b;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    margin-bottom: 0.6rem;
}
.toggle-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 0.85rem;
    color: #cbd5e1;
}
.toggle { position: relative; display: inline-block; width: 36px; height: 20px; flex-shrink: 0; }
.toggle input { opacity: 0; width: 0; height: 0; }
.toggle-slider {
    position: absolute;
    cursor: pointer;
    inset: 0;
    background: #334155;
    border-radius: 20px;
    transition: background 0.2s;
}
.toggle-slider:before {
    content: '';
    position: absolute;
    height: 14px;
    width: 14px;
    left: 3px;
    top: 3px;
    background: #fff;
    border-radius: 50%;
    transition: transform 0.2s;
}
.toggle input:checked + .toggle-slider { background: #3b82f6; }
.toggle input:checked + .toggle-slider:before { transform: translateX(16px); }

/* ---- Inline project control buttons ---- */
.proj-ctrl {
    font-size: 0.9rem;
    padding: 0.2rem 0.35rem;
    opacity: 0;
    transition: opacity 0.15s;
}
.proj-header:hover .proj-ctrl { opacity: 1; }
.proj-ctrl:focus { opacity: 1; }

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
.ver-chip-link { color: #60a5fa; text-decoration: none; }
.ver-chip-link:hover { text-decoration: underline; }
.ver-more-list { display: none; border-top: 1px solid #334155; }
.ver-more-list.open { display: block; }

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
.proj-row {
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
.webhook-panel { border-top: 1px solid #334155; padding: 0.75rem 1.1rem; background: #0f172a; }
.webhook-item { display: flex; align-items: center; gap: 0.75rem; padding: 0.35rem 0; border-bottom: 1px solid #1a2535; }
.webhook-item:last-child { border-bottom: none; }
.webhook-url { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 0.8rem; color: #94a3b8; }
.webhook-add { display: flex; gap: 0.5rem; margin-top: 0.6rem; flex-wrap: wrap; align-items: center; }
.webhook-input { padding: 0.4rem 0.6rem; background: #1e293b; border: 1px solid #334155; border-radius: 4px; color: #e2e8f0; font-size: 0.8rem; }
.webhook-input:focus { outline: none; border-color: #3b82f6; }

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
        <div class="form-group">
            <label>Email</label>
            <input type="email" name="email" required autocomplete="email">
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
        <button class="btn btn-ghost" onclick="openAccountPanel()" title="Account settings" style="font-size:1rem;padding:0.3rem 0.5rem">&#x2699;</button>
        <button class="btn btn-ghost" id="notif-btn" onclick="toggleNotifications()" title="Enable push notifications" style="font-size:1rem;padding:0.3rem 0.5rem">&#x1F514;</button>
        <button class="btn btn-ghost" onclick="doLogout()">Sign out</button>
    </div>
</header>

<div id="toast" class="toast"></div>
<div id="verify-banner" style="display:none;background:#422006;color:#fbbf24;border-bottom:1px solid #92400e;padding:0.6rem 1.1rem;font-size:0.85rem">
    Please verify your email address.
    <button onclick="doResendVerification()" style="background:none;border:none;color:#fbbf24;text-decoration:underline;cursor:pointer;font-size:0.85rem;margin-left:0.5rem">Resend verification email</button>
    <button onclick="dismissVerifyBanner()" style="background:none;border:none;color:#fbbf24;cursor:pointer;float:right;font-size:1rem">&#x2715;</button>
</div>

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
        <div id="settings-panel" class="settings-panel">
            <div class="settings-panel-inner">
                <div class="add-panel-title">
                    <span id="settings-panel-title">Settings</span>
                    <button class="btn btn-ghost" onclick="closeSettingsPanel()" style="font-size:1rem;padding:0.2rem 0.4rem">&#x2715;</button>
                </div>
                <div id="settings-panel-body"></div>
            </div>
        </div>
        <div id="account-panel" class="account-panel">
            <div class="account-panel-inner">
                <div class="add-panel-title">
                    Account Settings
                    <button class="btn btn-ghost" onclick="closeAccountPanel()" style="font-size:1rem;padding:0.2rem 0.4rem">&#x2715;</button>
                </div>
                <div id="account-panel-body"></div>
            </div>
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
    var w = Math.floor(d / 7);
    var mo = Math.floor(d / 30);
    if (m < 2)   return 'just now';
    if (m < 60)  return m + ' min' + (m === 1 ? '' : 's') + ' ago';
    if (h < 24)  return h + ' hr' + (h === 1 ? '' : 's') + ' ago';
    if (d < 7)   return d + ' day' + (d === 1 ? '' : 's') + ' ago';
    if (d < 30)  return w + ' wk' + (w === 1 ? '' : 's') + ' ago';
    return mo + ' mth' + (mo === 1 ? '' : 's') + ' ago';
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
    updateVerifyBanner();
    fetch('/api/refresh-check').catch(function() {});
    initNotifications();
    loadReleases();
}

function updateVerifyBanner() {
    var banner = document.getElementById('verify-banner');
    if (currentUser && currentUser.smtp_enabled && !currentUser.email_verified && currentUser.email) {
        banner.style.display = '';
    } else {
        banner.style.display = 'none';
    }
}

function dismissVerifyBanner() {
    document.getElementById('verify-banner').style.display = 'none';
}

function doResendVerification() {
    fetch('/api/resend-verification', { method: 'POST' })
        .then(function(r) {
            if (r.status === 429) throw new Error('Too many attempts. Try again later.');
            if (!r.ok) throw new Error('Failed to resend.');
            return r.json();
        })
        .then(function() {
            toast('Verification email sent! Check your inbox.', 'ok');
        })
        .catch(function(err) {
            toast(err.message, 'err');
        });
}

function init() {
    var params = new URLSearchParams(window.location.search);
    if (params.get('verified') === '1' || params.get('verify_error') === '1') {
        history.replaceState(null, '', '/');
    }
    fetch('/api/me')
        .then(function(r) {
            if (!r.ok) throw new Error('not authenticated');
            return r.json();
        })
        .then(function(user) {
            currentUser = user;
            showApp();
            if (params.get('verified') === '1') {
                toast('Email verified!', 'ok');
            } else if (params.get('verify_error') === '1') {
                toast('Verification link expired or invalid. Please request a new one.', 'err');
            }
        })
        .catch(function() {
            document.getElementById('auth-page').style.display = '';
            document.getElementById('app-page').style.display = 'none';
        });
}

// ---- Add panel ----
function toggleAddPanel() {
    closeSettingsPanel();
    closeAccountPanel();
    var panel = document.getElementById('add-panel');
    panel.classList.toggle('open');
}

function closeAddPanel() {
    document.getElementById('add-panel').classList.remove('open');
}

// ---- Account panel ----
function openAccountPanel() {
    closeAddPanel();
    closeSettingsPanel();
    document.getElementById('account-panel').classList.add('open');
    loadAccountPanel();
}

function closeAccountPanel() {
    document.getElementById('account-panel').classList.remove('open');
}

function loadAccountPanel() {
    var body = document.getElementById('account-panel-body');

    // Show loading indicator
    body.textContent = '';
    var loading = document.createElement('div');
    loading.style.cssText = 'font-size:0.85rem;color:#64748b';
    loading.textContent = 'Loading...';
    body.appendChild(loading);

    fetch('/api/account-settings')
        .then(function(r) { return r.json(); })
        .then(function(settings) {
            var smtpOk = !!(currentUser && currentUser.smtp_enabled);
            var verifiedOk = !!(currentUser && currentUser.email_verified);
            var canUse = smtpOk && verifiedOk;
            var title = !smtpOk ? 'Email not configured' : (!verifiedOk ? 'Verify your email to enable' : '');

            body.textContent = '';

            // Username row
            var userSection = document.createElement('div');
            userSection.style.marginBottom = '1rem';
            var userLabel = document.createElement('div');
            userLabel.style.cssText = 'font-size:0.8rem;color:#64748b;margin-bottom:0.25rem';
            userLabel.textContent = 'Username';
            var userValue = document.createElement('div');
            userValue.style.fontSize = '0.9rem';
            userValue.textContent = currentUser ? currentUser.username : '';
            userSection.appendChild(userLabel);
            userSection.appendChild(userValue);
            body.appendChild(userSection);

            // Daily email summary toggle row
            var row = document.createElement('div');
            row.style.cssText = 'display:flex;align-items:center;justify-content:space-between;padding:0.6rem 0;border-top:1px solid #334155';

            var lbl = document.createElement('label');
            lbl.htmlFor = 'digest-toggle';
            lbl.style.cssText = 'font-size:0.85rem;cursor:' + (canUse ? 'pointer' : 'default');
            lbl.textContent = 'Daily email summary';

            var chk = document.createElement('input');
            chk.type = 'checkbox';
            chk.id = 'digest-toggle';
            chk.checked = !!settings.email_digest;
            chk.disabled = !canUse;
            if (title) { chk.title = title; }
            chk.onchange = function() { toggleEmailDigest(this); };

            row.appendChild(lbl);
            row.appendChild(chk);
            body.appendChild(row);

            var rssToken = currentUser && currentUser.rss_token;
            if (rssToken) {
                var rssSection = document.createElement('div');
                rssSection.style.cssText = 'padding:0.75rem 0;border-top:1px solid #334155';
                var rssLabel = document.createElement('div');
                rssLabel.style.cssText = 'font-size:0.8rem;color:#64748b;margin-bottom:0.4rem';
                rssLabel.textContent = 'RSS Feed';
                rssSection.appendChild(rssLabel);
                var rssRow = document.createElement('div');
                rssRow.style.cssText = 'display:flex;gap:0.4rem;align-items:center';
                var feedURL = window.location.origin + '/feed/' + rssToken;
                var rssInput = document.createElement('input');
                rssInput.type = 'text';
                rssInput.readOnly = true;
                rssInput.value = feedURL;
                rssInput.style.cssText = 'flex:1;font-size:0.75rem;padding:0.3rem 0.4rem;background:#1e293b;border:1px solid #334155;color:#94a3b8;border-radius:4px;min-width:0';
                rssRow.appendChild(rssInput);
                var copyBtn = document.createElement('button');
                copyBtn.className = 'btn btn-ghost';
                copyBtn.style.cssText = 'font-size:0.75rem;padding:0.3rem 0.5rem;flex-shrink:0';
                copyBtn.textContent = 'Copy';
                copyBtn.onclick = function() {
                    navigator.clipboard.writeText(feedURL).then(function() {
                        copyBtn.textContent = 'Copied!';
                        setTimeout(function() { copyBtn.textContent = 'Copy'; }, 1500);
                    });
                };
                rssRow.appendChild(copyBtn);
                rssSection.appendChild(rssRow);
                body.appendChild(rssSection);
            }
        })
        .catch(function() {
            body.textContent = '';
            var err = document.createElement('div');
            err.style.cssText = 'color:#f87171;font-size:0.85rem';
            err.textContent = 'Failed to load settings.';
            body.appendChild(err);
        });
}

function toggleEmailDigest(checkbox) {
    fetch('/api/account-settings', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({email_digest: checkbox.checked})
    }).then(function(r) {
        if (!r.ok) return r.text().then(function(t) { throw new Error(t || 'Failed to save'); });
        toast('Settings saved', 'ok');
    }).catch(function() {
        checkbox.checked = !checkbox.checked;
        toast('Failed to save settings', 'error');
    });
}

document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') { closeAddPanel(); closeSettingsPanel(); closeAccountPanel(); }
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

        var visible = rels.slice(0, 5);
        var hidden  = rels.slice(5);

        var chips = visible.map(function(r, i) {
            var notes = r.release_notes || r.description || '';
            var vlink = '<a class="ver-chip-link" href="' + esc(r.url) + '" target="_blank"' +
                (notes ? ' title="' + esc(notes) + '"' : '') + '>' + esc(r.version) + '</a>';
            return fmtAge(r.published_at) + ': ' + vlink +
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
    }).join('');
}

function toggleMore(btn) {
    var list = btn.closest('.ver-chips').nextElementSibling;
    var open = list.classList.toggle('open');
    btn.textContent = open ? '\u25B4 less' : '+' + btn.dataset.count + ' more \u25BE';
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
        if (_settingsRepoID) {
            var fresh = (res[1] || []).filter(function(p) { return p.id === _settingsRepoID; })[0];
            if (fresh) {
                loadSettingsPanel(_settingsRepoID, fresh.push_enabled);
            } else {
                closeSettingsPanel();
            }
        }
    }).catch(function(err) {
        document.getElementById('releases-root').innerHTML = '<div class="empty"><h3>Failed to load</h3></div>';
        console.error('loadReleases:', err);
    });
}

// ---- Settings panel ----
var _settingsRepoID = null;

function openSettingsPanel(btn) {
    var id = btn.dataset.id;
    var name = btn.dataset.name;
    var pushEnabled = btn.dataset.push === '1';
    _settingsRepoID = id;
    document.getElementById('settings-panel-title').textContent = name;
    closeAddPanel();
    closeAccountPanel();
    document.getElementById('settings-panel').classList.add('open');
    loadSettingsPanel(id, pushEnabled);
}

function closeSettingsPanel() {
    document.getElementById('settings-panel').classList.remove('open');
    _settingsRepoID = null;
}

function loadSettingsPanel(id, pushEnabled) {
    var body = document.getElementById('settings-panel-body');
    body.innerHTML = '<div style="font-size:0.85rem;color:#64748b">Loading...</div>';
    fetch('/api/webhooks?repo_id=' + encodeURIComponent(id))
        .then(function(r) { return r.json(); })
        .then(function(hooks) {
            body.innerHTML = renderSettingsPanelBody(id, pushEnabled, hooks || []);
        })
        .catch(function() {
            body.innerHTML = '<div style="color:#f87171;font-size:0.85rem">Failed to load settings.</div>';
        });
}

function renderSettingsPanelBody(id, pushEnabled, hooks) {
    return '<div class="settings-section">' +
            '<div class="settings-section-title">Push Notifications</div>' +
            '<div class="toggle-row">' +
                '<span>Notify on new releases</span>' +
                '<label class="toggle">' +
                    '<input type="checkbox" ' + (pushEnabled ? 'checked' : '') + ' onchange="doTogglePush(this,\'' + esc(id) + '\')">' +
                    '<span class="toggle-slider"></span>' +
                '</label>' +
            '</div>' +
        '</div>' +
        '<div class="settings-section">' +
            '<div class="settings-section-title">Webhooks</div>' +
            '<div id="whpanel-' + esc(id) + '">' + renderWebhookPanel(id, hooks) + '</div>' +
        '</div>';
}

function doTogglePush(checkbox, id) {
    fetch('/api/project-settings', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({repo_id: id, push_enabled: checkbox.checked})
    }).then(function(r) {
        if (r.ok) {
            toast('Notification setting updated.', 'ok');
        } else {
            checkbox.checked = !checkbox.checked;
            toast('Failed to update setting.', 'err');
        }
    }).catch(function() {
        checkbox.checked = !checkbox.checked;
        toast('Error updating setting.', 'err');
    });
}

// ---- Projects tab ----
function buildProjectsHTML(projects) {
    if (!projects || projects.length === 0) {
        return '<div class="empty"><h3>No projects tracked</h3><p>Use "+ Add Project" to get started.</p></div>';
    }
    return projects.map(function(p) {
        var sub = p.last_refresh ? 'Refreshed ' + fmtAge(p.last_refresh) : 'Never refreshed';
        return '<div class="card" id="projcard-' + esc(p.id) + '">' +
            '<div class="proj-row">' +
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
                    '<button class="btn btn-outline" data-id="' + esc(p.id) + '" id="whbtn-' + esc(p.id) + '" onclick="toggleWebhooks(this)">Webhooks</button>' +
                    '<button class="btn btn-red" data-id="' + esc(p.id) + '" data-name="' + esc(p.name) + '" onclick="doDelete(this)">Delete</button>' +
                '</div>' +
            '</div>' +
            '<div class="webhook-panel" id="whpanel-' + esc(p.id) + '" style="display:none"></div>' +
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
                setTimeout(function() { loadReleases(); }, 2000);
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

// ---- Webhooks ----
function toggleWebhooks(btn) {
    var id = btn.dataset.id;
    var panel = document.getElementById('whpanel-' + id);
    if (panel.style.display === 'none') {
        panel.style.display = 'block';
        btn.style.color = '#60a5fa';
        loadWebhooks(id);
    } else {
        panel.style.display = 'none';
        btn.style.color = '';
    }
}

function loadWebhooks(repoID) {
    var panel = document.getElementById('whpanel-' + repoID);
    if (!panel) return;
    panel.innerHTML = '<div style="font-size:0.8rem;color:#475569;padding:0.25rem 0">Loading...</div>';
    fetch('/api/webhooks?repo_id=' + encodeURIComponent(repoID))
        .then(function(r) { return r.json(); })
        .then(function(hooks) { panel.innerHTML = renderWebhookPanel(repoID, hooks || []); })
        .catch(function() { panel.innerHTML = '<div style="color:#f87171;font-size:0.8rem">Failed to load webhooks.</div>'; });
}

function renderWebhookPanel(repoID, hooks) {
    var list = hooks.length ? hooks.map(function(h) {
        var label = h.has_secret ? ' <span style="color:#64748b;font-size:0.7rem">[signed]</span>' : '';
        return '<div class="webhook-item">' +
            '<span class="webhook-url">' + esc(h.url) + label + '</span>' +
            '<button class="btn btn-ghost" style="color:#f87171;font-size:0.75rem;padding:0.2rem 0.5rem" ' +
                'data-wid="' + esc(h.id) + '" data-repo="' + esc(repoID) + '" onclick="doDeleteWebhook(this)">Remove</button>' +
        '</div>';
    }).join('') : '<div style="font-size:0.8rem;color:#475569;padding:0.25rem 0 0.5rem">No webhooks configured.</div>';

    return '<div>' + list + '</div>' +
        '<form class="webhook-add" onsubmit="doAddWebhook(event,\'' + esc(repoID) + '\')">' +
            '<input class="webhook-input" type="url" name="url" required placeholder="https://your-endpoint.com/webhook" style="flex:1;min-width:180px">' +
            '<input class="webhook-input" type="text" name="secret" placeholder="Secret (optional)" style="width:150px">' +
            '<button type="submit" class="btn btn-primary" style="font-size:0.78rem">Add</button>' +
        '</form>';
}

function doAddWebhook(e, repoID) {
    e.preventDefault();
    var data = {repo_id: repoID};
    new FormData(e.target).forEach(function(v, k) { if (v) data[k] = v; });
    fetch('/api/webhooks', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(data)
    }).then(function(r) {
        if (!r.ok) return r.text().then(function(t) { throw new Error(t.trim()); });
        e.target.reset();
        loadWebhooks(repoID);
        toast('Webhook added.', 'ok');
    }).catch(function(err) { toast('Error: ' + err, 'err'); });
}

function doDeleteWebhook(btn) {
    var wid = btn.dataset.wid;
    var repoID = btn.dataset.repo;
    fetch('/api/webhooks?id=' + encodeURIComponent(wid), {method: 'DELETE'})
        .then(function(r) {
            if (r.ok) { loadWebhooks(repoID); toast('Webhook removed.'); }
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
        onPlatformChange();
        toast('Project added — fetching releases in background.');
        closeAddPanel();
        loadReleases();
    }).catch(function(err) {
        toast('Error: ' + err, 'err');
    }).finally(function() {
        btn.disabled = false; btn.textContent = 'Add Project';
    });
});

// ---- Push notifications ----
var _pushSub = null;

function urlBase64ToUint8Array(b64) {
    var pad = '='.repeat((4 - b64.length % 4) % 4);
    var base64 = (b64 + pad).replace(/-/g, '+').replace(/_/g, '/');
    var raw = atob(base64);
    var arr = new Uint8Array(raw.length);
    for (var i = 0; i < raw.length; i++) arr[i] = raw.charCodeAt(i);
    return arr;
}

function updateNotifBtn() {
    var btn = document.getElementById('notif-btn');
    if (!btn) return;
    var denied = Notification.permission === 'denied';
    if (denied) {
        btn.title = 'Notifications blocked — check browser site settings';
        btn.style.opacity = '0.35';
    } else if (_pushSub) {
        btn.title = 'Push notifications on — click to disable';
        btn.style.opacity = '1';
        btn.style.color = '#fbbf24';
    } else {
        btn.title = 'Enable push notifications';
        btn.style.opacity = '0.6';
        btn.style.color = '';
    }
}

function initNotifications() {
    var btn = document.getElementById('notif-btn');
    if (!('serviceWorker' in navigator) || !('PushManager' in window)) {
        if (btn) btn.style.display = 'none';
        return;
    }
    navigator.serviceWorker.register('/sw.js').then(function(reg) {
        return reg.pushManager.getSubscription();
    }).then(function(sub) {
        _pushSub = sub;
        updateNotifBtn();
    }).catch(function(err) {
        console.warn('SW init failed:', err);
        if (btn) btn.style.display = 'none';
    });
}

function toggleNotifications() {
    if (Notification.permission === 'denied') {
        toast('Notifications are blocked. Enable them in your browser site settings.', 'err');
        return;
    }
    if (_pushSub) {
        var endpoint = _pushSub.endpoint;
        _pushSub.unsubscribe().then(function() {
            _pushSub = null;
            return fetch('/api/push/subscribe', {
                method: 'DELETE',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({endpoint: endpoint})
            });
        }).then(function() {
            updateNotifBtn();
            toast('Push notifications disabled.', 'inf');
        }).catch(function(err) {
            toast('Error: ' + err, 'err');
        });
        return;
    }
    Notification.requestPermission().then(function(perm) {
        if (perm !== 'granted') {
            updateNotifBtn();
            toast('Notification permission denied.', 'err');
            return;
        }
        fetch('/api/push/vapid-key').then(function(r) { return r.json(); }).then(function(data) {
            return navigator.serviceWorker.ready.then(function(reg) {
                return reg.pushManager.subscribe({
                    userVisibleOnly: true,
                    applicationServerKey: urlBase64ToUint8Array(data.publicKey)
                });
            });
        }).then(function(sub) {
            _pushSub = sub;
            var j = sub.toJSON();
            return fetch('/api/push/subscribe', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({endpoint: j.endpoint, keys: j.keys})
            });
        }).then(function() {
            updateNotifBtn();
            toast('Push notifications enabled!', 'ok');
        }).catch(function(err) {
            toast('Failed to enable notifications: ' + err, 'err');
        });
    });
}

// ---- Boot ----
onPlatformChange();
init();
</script>
</body>
</html>
`

var tmpl = template.Must(template.New("").Parse(htmlTemplate))
