// ---- Utilities ----
function esc(s) {
    var d = document.createElement('div');
    d.textContent = s == null ? '' : String(s);
    return d.innerHTML.replace(/"/g, '&quot;').replace(/'/g, '&#39;');
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
        allProjects = [];
        allReleases = [];
        currentPage = 1;
        document.getElementById('app-page').style.display = 'none';
        document.getElementById('auth-page').style.display = '';
        document.getElementById('auth-error').style.display = 'none';
    });
}

// ---- App init ----
var currentUser = null;
var allProjects = [];
var allReleases = [];
var currentPage = 1;
var pageSize = parseInt(localStorage.getItem('page_size'), 10) || 10;
var SORT_MODES = ['latest', 'added', 'name'];
var sortMode = (function() {
    try { var m = localStorage.getItem('sort_mode'); if (SORT_MODES.indexOf(m) !== -1) return m; } catch (e) {}
    return 'latest';
})();

function showApp() {
    document.getElementById('auth-page').style.display = 'none';
    document.getElementById('app-page').style.display = '';
    document.getElementById('username-display').textContent = currentUser.email;
    updateVerifyBanner();
    fetch('/api/refresh-check').catch(function() {});
    initNotifications();
    if (currentUser.page_size && [5, 10, 20].indexOf(currentUser.page_size) !== -1) {
        pageSize = currentUser.page_size;
        localStorage.setItem('page_size', String(pageSize));
    }
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

// ---- Project detail popup ----
function openProjectDetail(id) {
    var proj = allProjects.filter(function(p) { return p.id === id; })[0] || {};
    var rels = allReleases.filter(function(r) { return r.project_id === id; })
        .sort(function(a, b) { return Date.parse(b.published_at) - Date.parse(a.published_at); })
        .slice(0, 10);
    var rows = rels.map(function(r) {
        var notes = r.release_notes || r.description || '';
        return '<div class="detail-ver">' +
            '<div class="ver-top"><a class="ver-link" href="' + esc(r.url) + '" target="_blank" rel="noopener noreferrer">' + esc(r.version) + '</a>' +
            (r.prerelease ? ' <span class="pre-tag">pre</span>' : '') +
            '<span class="ver-date">' + fmtAge(r.published_at) + '</span></div>' +
            (notes ? '<div class="detail-notes">' + esc(notes) + '</div>' : '') +
        '</div>';
    }).join('') || '<p class="detail-empty">No versions yet.</p>';
    document.getElementById('detail-body').innerHTML =
        '<div class="add-panel-title"><span>' + projIconHTML(proj, proj.name || '', proj.platform || '') + ' ' + esc(proj.name || '') +
        (proj.platform ? ' <span class="badge ' + esc(proj.platform) + '">' + esc(proj.platform) + '</span>' : '') + '</span>' +
        '<button class="btn btn-ghost" aria-label="Close" onclick="document.getElementById(\'detail-dialog\').close()">&#x2715;</button></div>' +
        '<div class="settings-section-title">Last ' + rels.length + ' version' + (rels.length === 1 ? '' : 's') + '</div>' + rows;
    document.getElementById('detail-dialog').showModal();
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
    loading.style.cssText = 'font-size:0.85rem;color:#8796ad';
    loading.textContent = 'Loading...';
    body.appendChild(loading);

    Promise.all([
        fetch('/api/account-settings').then(apiJSON),
        fetch('/api/gitlab-settings').then(apiJSON)
    ])
        .then(function(res) {
            var settings = res[0];
            var gitlabSettings = res[1];
            var smtpOk = !!(currentUser && currentUser.smtp_enabled);
            var verifiedOk = !!(currentUser && currentUser.email_verified);
            var canUse = smtpOk && verifiedOk;
            var title = !smtpOk ? 'Email not configured' : (!verifiedOk ? 'Verify your email to enable' : '');

            body.textContent = '';

            // Username row
            var userSection = document.createElement('div');
            userSection.style.marginBottom = '1rem';
            var userLabel = document.createElement('div');
            userLabel.style.cssText = 'font-size:0.8rem;color:#8796ad;margin-bottom:0.25rem';
            userLabel.textContent = 'Email';
            var userValue = document.createElement('div');
            userValue.style.fontSize = '0.9rem';
            userValue.textContent = currentUser ? currentUser.email : '';
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
                rssLabel.style.cssText = 'font-size:0.8rem;color:#8796ad;margin-bottom:0.4rem';
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
                    }).catch(function() {
                        toast('Failed to copy — please copy manually', 'err');
                    });
                };
                rssRow.appendChild(copyBtn);
                rssSection.appendChild(rssRow);
                body.appendChild(rssSection);
            }

            var glSection = document.createElement('div');
            glSection.style.cssText = 'padding:0.75rem 0;border-top:1px solid #334155';
            var glLabel = document.createElement('div');
            glLabel.style.cssText = 'font-size:0.8rem;color:#8796ad;margin-bottom:0.4rem';
            glLabel.textContent = 'GitLab Sync';
            glSection.appendChild(glLabel);

            var glUrlInput = document.createElement('input');
            glUrlInput.type = 'url';
            glUrlInput.id = 'gitlab-url-input';
            glUrlInput.placeholder = 'https://gitlab.example.com';
            glUrlInput.value = gitlabSettings.gitlab_url || '';
            glUrlInput.className = 'webhook-input';
            glUrlInput.style.cssText = 'width:100%;margin-bottom:0.4rem;box-sizing:border-box';
            glSection.appendChild(glUrlInput);

            var glTokenInput = document.createElement('input');
            glTokenInput.type = 'password';
            glTokenInput.id = 'gitlab-token-input';
            glTokenInput.placeholder = gitlabSettings.has_token ? 'Token saved (enter to replace)' : 'API token';
            glTokenInput.className = 'webhook-input';
            glTokenInput.style.cssText = 'width:100%;margin-bottom:0.4rem;box-sizing:border-box';
            glSection.appendChild(glTokenInput);

            var glGroupInput = document.createElement('input');
            glGroupInput.type = 'text';
            glGroupInput.id = 'gitlab-group-input';
            glGroupInput.placeholder = 'Upstream group (optional, e.g. UpStream)';
            glGroupInput.value = gitlabSettings.gitlab_group || '';
            glGroupInput.className = 'webhook-input';
            glGroupInput.style.cssText = 'width:100%;margin-bottom:0.4rem;box-sizing:border-box';
            glSection.appendChild(glGroupInput);

            var glGroupHint = document.createElement('div');
            glGroupHint.style.cssText = 'font-size:0.72rem;color:#8796ad;margin:-0.2rem 0 0.4rem';
            glGroupHint.textContent = 'Mirrors go to <group>/<upstream-owner>/<repo> instead of your personal namespace. The group must already exist in GitLab.';
            glSection.appendChild(glGroupHint);

            var glSaveBtn = document.createElement('button');
            glSaveBtn.className = 'btn btn-primary';
            glSaveBtn.style.cssText = 'font-size:0.78rem';
            glSaveBtn.textContent = 'Save GitLab Settings';
            glSaveBtn.onclick = function() { saveGitLabSettings(); };
            glSection.appendChild(glSaveBtn);

            var awesomeRow = document.createElement('div');
            awesomeRow.style.cssText = 'display:flex;align-items:center;justify-content:space-between;padding:0.6rem 0;margin-top:0.6rem;border-top:1px solid #1a2535';
            var awesomeLbl = document.createElement('label');
            awesomeLbl.htmlFor = 'awesome-toggle';
            awesomeLbl.style.cssText = 'font-size:0.85rem;cursor:pointer';
            awesomeLbl.textContent = 'Generate Awesome page';
            var awesomeChk = document.createElement('input');
            awesomeChk.type = 'checkbox';
            awesomeChk.id = 'awesome-toggle';
            awesomeChk.checked = !!gitlabSettings.awesome_enabled;
            awesomeChk.onchange = function() { onAwesomeToggle(this); };
            awesomeRow.appendChild(awesomeLbl);
            awesomeRow.appendChild(awesomeChk);
            glSection.appendChild(awesomeRow);

            var awesomeNameInput = document.createElement('input');
            awesomeNameInput.type = 'text';
            awesomeNameInput.id = 'awesome-repo-name-input';
            awesomeNameInput.className = 'webhook-input';
            awesomeNameInput.placeholder = 'Repo name (e.g. my-awesome-repos)';
            awesomeNameInput.value = gitlabSettings.awesome_repo_name || '';
            awesomeNameInput.style.cssText = 'width:100%;margin-top:0.3rem;box-sizing:border-box;display:' + (gitlabSettings.awesome_enabled ? 'block' : 'none');
            glSection.appendChild(awesomeNameInput);

            body.appendChild(glSection);
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
        toast('Failed to save settings', 'err');
    });
}

function saveGitLabSettings() {
    var url = document.getElementById('gitlab-url-input').value.trim();
    var token = document.getElementById('gitlab-token-input').value;
    var group = document.getElementById('gitlab-group-input').value.trim();
    if (!url || !token) { toast('GitLab URL and token are required', 'err'); return; }
    fetch('/api/gitlab-settings', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({gitlab_url: url, gitlab_token: token, gitlab_group: group})
    }).then(function(r) {
        if (!r.ok) return r.text().then(function(t) { throw new Error(t.trim()); });
        toast('GitLab settings saved', 'ok');
        document.getElementById('gitlab-token-input').value = '';
        document.getElementById('gitlab-token-input').placeholder = 'Token saved (enter to replace)';
    }).catch(function(err) { toast('Error: ' + err, 'err'); });
}

function onAwesomeToggle(checkbox) {
    var nameInput = document.getElementById('awesome-repo-name-input');
    var repoName = nameInput.value.trim();
    if (checkbox.checked && !repoName) {
        toast('Enter a repo name for the Awesome page', 'err');
        checkbox.checked = false;
        return;
    }
    nameInput.style.display = checkbox.checked ? 'block' : 'none';
    fetch('/api/gitlab-settings/awesome', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({enabled: checkbox.checked, repo_name: repoName})
    }).then(function(r) {
        if (!r.ok) return r.text().then(function(t) { throw new Error(t.trim()); });
        return r.json().then(function(body) {
            if (body.warning) {
                toast('Awesome page settings saved, but sync failed: ' + body.warning, 'err');
                return;
            }
            toast(checkbox.checked ? 'Awesome page enabled' : 'Awesome page disabled', 'ok');
        });
    }).catch(function(err) {
        checkbox.checked = !checkbox.checked;
        nameInput.style.display = checkbox.checked ? 'block' : 'none';
        toast('Error: ' + err, 'err');
    });
}

document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') { closeAddPanel(); closeSettingsPanel(); closeAccountPanel(); }
    if ((e.key === 'Enter' || e.key === ' ') && e.target.classList && e.target.classList.contains('ver-row')) {
        e.preventDefault();
        toggleNotes(e.target);
    }
});

// Parse a JSON API response; expired session returns to sign-in, other failures throw.
function apiJSON(r) {
    if (r.status === 401) { doLogout(); throw new Error('Session expired. Please sign in again.'); }
    if (!r.ok) throw new Error('Request failed (' + r.status + ')');
    return r.json();
}

// ---- Releases tab ----
// Real icon when the server has one cached, else a letter badge in the platform's colour.
function projIconHTML(proj, name, platform) {
    var letter = ((name || '?').trim().charAt(0) || '?').toUpperCase();
    if (proj && proj.icon_version) {
        return '<img class="proj-icon" src="/api/icon?id=' + encodeURIComponent(proj.id) + '&v=' + Number(proj.icon_version) + '" ' +
            'alt="" width="22" height="22" data-letter="' + esc(letter) + '" data-platform="' + esc(platform) + '" ' +
            'onerror="iconFallback(this)">';
    }
    return '<span class="proj-icon letter ' + esc(platform) + '" aria-hidden="true">' + esc(letter) + '</span>';
}

function iconFallback(img) {
    var span = document.createElement('span');
    span.className = 'proj-icon letter ' + (img.dataset.platform || '');
    span.setAttribute('aria-hidden', 'true');
    span.textContent = img.dataset.letter || '?';
    img.replaceWith(span);
}

function buildReleasesHTML(releases, projects) {
    if (!releases || releases.length === 0) {
        return '<div class="empty"><h3>No releases yet</h3><p>Add projects to start tracking.</p></div>';
    }
    var projMap = {};
    (projects || []).forEach(function(p) { projMap[p.id] = p; });

    var groups = {};
    releases.forEach(function(r) {
        var pid = r.project_id || '';
        (groups[pid] = groups[pid] || []).push(r);
    });
    // Follow the (sorted) project order; any release whose project is unknown goes last.
    var order = (projects || []).map(function(p) { return p.id; })
        .filter(function(id) { return groups[id]; });
    Object.keys(groups).forEach(function(id) { if (order.indexOf(id) === -1) order.push(id); });

    return order.map(function(pid) {
        var proj = projMap[pid];
        var pname = proj ? proj.name : 'Unknown';
        var platform = proj ? proj.platform : '';
        var backupUrl = proj && proj.gitlab_project_path ? proj.gitlab_project_path.replace(/\.git$/, '') : '';
        var backupLink = (proj && proj.gitlab_sync_enabled && /^https?:\/\//i.test(backupUrl)) ?
            '<a href="' + esc(backupUrl) + '" target="_blank" rel="noopener noreferrer" class="badge">[BACKUP]</a>' : '';
        var rels = groups[pid];

        var visible = rels.slice(0, 5);
        var hidden  = rels.slice(5);

        var chips = visible.map(function(r, i) {
            var notes = r.release_notes || r.description || '';
            var vlink = '<a class="ver-chip-link" href="' + esc(r.url) + '" target="_blank"' +
                (notes ? ' title="' + esc(notes) + '"' : '') + '>' + esc(r.version) + '</a>';
            return '<span class="ver-chip">' + fmtAge(r.published_at) + ': ' + vlink +
                (r.prerelease ? ' <span class="pre-tag">pre</span>' : '') +
                (i < visible.length - 1 || hidden.length > 0 ? ',' : '') + '</span>';
        }).join('');

        var moreBtn = hidden.length > 0
            ? '<button class="ver-chip-more" data-count="' + hidden.length + '" ' +
              'onclick="toggleMore(this)">+' + hidden.length + ' more \u25BE</button>'
            : '';

        var allRows = rels.map(function(r) {
            var notes = r.release_notes || r.description || '';
            return '<div class="ver-row" role="button" tabindex="0" onclick="toggleNotes(this)">' +
                '<div class="ver-top">' +
                    '<span class="ver-tag">' + esc(r.version) + (r.prerelease ? ' <span class="pre-tag">pre</span>' : '') + '</span>' +
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
                    projIconHTML(proj, pname, platform) +
                    '<button class="proj-name proj-name-btn" title="Show versions" data-id="' + esc(pid) + '" onclick="openProjectDetail(this.dataset.id)">' + esc(pname) + '</button>' +
                    (platform ? '<span class="badge ' + esc(platform) + '">' + esc(platform) + '</span>' : '') +
                    backupLink +
                '</div>' +
                '<div class="proj-right">' +
                    '<button class="btn btn-ghost proj-ctrl" title="Refresh" aria-label="Refresh ' + esc(pname) + '" ' +
                        'data-id="' + esc(pid) + '" ' +
                        'onclick="doRefresh(this)">&#x21BB;</button>' +
                    '<button class="btn btn-ghost proj-ctrl" title="Settings" aria-label="Settings for ' + esc(pname) + '" ' +
                        'data-id="' + esc(pid) + '" data-name="' + esc(pname) + '" ' +
                        'data-push="' + (proj && proj.push_enabled ? '1' : '0') + '" ' +
                        'data-gitlab-enabled="' + (proj && proj.gitlab_sync_enabled ? '1' : '0') + '" ' +
                        'data-gitlab-freq="' + esc((proj && proj.gitlab_sync_frequency) || 'daily') + '" ' +
                        'data-gitlab-error="' + esc((proj && proj.last_gitlab_sync_error) || '') + '" ' +
                        'data-email-immediate="' + (proj && proj.email_immediate ? '1' : '0') + '" ' +
                        'data-include-prereleases="' + (proj && proj.include_prereleases ? '1' : '0') + '" ' +
                        'onclick="openSettingsPanel(this)">&#x2699;</button>' +
                    '<button class="btn btn-ghost proj-ctrl" title="Delete" aria-label="Delete ' + esc(pname) + '" style="color:#f87171" ' +
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

// Sorts allProjects in place. Server returns projects oldest-added first, so
// that index (_idx, stamped on load) is the "date added" order.
function sortProjects() {
    var latest = {};
    allReleases.forEach(function(r) {
        var t = Date.parse(r.published_at) || 0;
        if (!(r.project_id in latest) || t > latest[r.project_id]) latest[r.project_id] = t;
    });
    var byName = function(a, b) { return a.name.toLowerCase().localeCompare(b.name.toLowerCase()); };
    allProjects.sort(function(a, b) {
        if (sortMode === 'name') return byName(a, b);
        if (sortMode === 'added') return b._idx - a._idx;
        return (latest[b.id] || 0) - (latest[a.id] || 0) || byName(a, b);
    });
}

function onSortChange(val) {
    if (SORT_MODES.indexOf(val) === -1) return;
    sortMode = val;
    try { localStorage.setItem('sort_mode', val); } catch (e) {}
    sortProjects();
    currentPage = 1;
    renderPage();
}

function renderPage() {
    var start = (currentPage - 1) * pageSize;
    var pageProjects = allProjects.slice(start, start + pageSize);
    var pageIDs = {};
    pageProjects.forEach(function(p) { pageIDs[p.id] = true; });
    var pageReleases = allReleases.filter(function(r) { return pageIDs[r.project_id]; });
    // All content is passed through esc() inside buildReleasesHTML and buildPaginationHTML.
    document.getElementById('releases-root').innerHTML =
        buildReleasesHTML(pageReleases, pageProjects) + buildPaginationHTML();
    var sel = document.getElementById('page-size-select');
    if (sel) sel.value = String(pageSize);
    var sortSel = document.getElementById('sort-select');
    if (sortSel) sortSel.value = sortMode;
}

function goToPage(n) {
    var totalPages = Math.ceil(allProjects.length / pageSize) || 1;
    if (n < 1 || n > totalPages) return;
    currentPage = n;
    renderPage();
}

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
        fetch('/api/releases').then(apiJSON),
        fetch('/api/projects').then(apiJSON)
    ]).then(function(res) {
        allReleases = res[0] || [];
        allProjects = res[1] || [];
        allProjects.forEach(function(p, i) { p._idx = i; });
        sortProjects();
        currentPage = 1;
        renderPage();
        if (_settingsRepoID) {
            var fresh = allProjects.filter(function(p) { return p.id === _settingsRepoID; })[0];
            if (fresh) {
                loadSettingsPanel(_settingsRepoID, fresh.push_enabled,
                    fresh.gitlab_sync_enabled, fresh.gitlab_sync_frequency || 'daily', fresh.last_gitlab_sync_error || '',
                    !!fresh.email_immediate, !!fresh.include_prereleases);
            } else {
                closeSettingsPanel();
                _settingsRepoID = null;
            }
        }
    }).catch(function(err) {
        document.getElementById('releases-root').innerHTML = '<div class="empty"><h3>Could not load releases</h3><p>' + esc(err.message) + '</p>' +
            '<button class="btn btn-outline" style="margin-top:0.75rem" onclick="loadReleases()">Try again</button></div>';
        console.error('loadReleases:', err);
    });
}

// ---- Settings panel ----
var _settingsRepoID = null;

function openSettingsPanel(btn) {
    var id = btn.dataset.id;
    var name = btn.dataset.name;
    var pushEnabled = btn.dataset.push === '1';
    var glEnabled = btn.dataset.gitlabEnabled === '1';
    var glFreq = btn.dataset.gitlabFreq || 'daily';
    var glError = btn.dataset.gitlabError || '';
    var emailImmediate = btn.dataset.emailImmediate === '1';
    var includePre = btn.dataset.includePrereleases === '1';
    _settingsRepoID = id;
    document.getElementById('settings-panel-title').textContent = name;
    closeAddPanel();
    closeAccountPanel();
    document.getElementById('settings-panel').classList.add('open');
    loadSettingsPanel(id, pushEnabled, glEnabled, glFreq, glError, emailImmediate, includePre);
}

function closeSettingsPanel() {
    document.getElementById('settings-panel').classList.remove('open');
    _settingsRepoID = null;
}

function loadSettingsPanel(id, pushEnabled, glEnabled, glFreq, glError, emailImmediate, includePre) {
    var body = document.getElementById('settings-panel-body');
    body.innerHTML = '<div style="font-size:0.85rem;color:#8796ad">Loading...</div>';
    fetch('/api/webhooks?repo_id=' + encodeURIComponent(id))
        .then(apiJSON)
        .then(function(hooks) {
            body.innerHTML = renderSettingsPanelBody(id, pushEnabled, hooks || [], glEnabled, glFreq, glError, includePre);
            addEmailToggleToPanel(id, !!emailImmediate);
        })
        .catch(function() {
            body.innerHTML = '<div style="color:#f87171;font-size:0.85rem">Failed to load settings.</div>';
        });
}

function renderSettingsPanelBody(id, pushEnabled, hooks, glEnabled, glFreq, glError, includePre) {
    return '<div class="settings-section">' +
            '<div class="settings-section-title">Push Notifications</div>' +
            '<div class="toggle-row">' +
                '<span>Notify on new releases</span>' +
                '<label class="toggle">' +
                    '<input type="checkbox" id="push-cb-' + esc(id) + '" ' + (pushEnabled ? 'checked' : '') + ' onchange="doTogglePush(this,\'' + esc(id) + '\')">' +
                    '<span class="toggle-slider"></span>' +
                '</label>' +
            '</div>' +
        '</div>' +
        '<div class="settings-section">' +
            '<div class="settings-section-title">Prereleases</div>' +
            '<div class="toggle-row">' +
                '<label for="pre-cb-' + esc(id) + '">Include prereleases</label>' +
                '<label class="toggle">' +
                    '<input type="checkbox" id="pre-cb-' + esc(id) + '" ' + (includePre ? 'checked' : '') + ' onchange="doTogglePrereleases(this,\'' + esc(id) + '\')">' +
                    '<span class="toggle-slider"></span>' +
                '</label>' +
            '</div>' +
            '<div class="settings-hint">Show alpha, beta and rc versions in the list, feed and digest, and notify about them.</div>' +
        '</div>' +
        '<div class="settings-section">' +
            '<div class="settings-section-title">GitLab Sync</div>' +
            renderGitLabSyncSection(id, glEnabled, glFreq, glError) +
        '</div>' +
        '<div class="settings-section">' +
            '<div class="settings-section-title">Webhooks</div>' +
            '<div id="whpanel-' + esc(id) + '">' + renderWebhookPanel(id, hooks) + '</div>' +
        '</div>';
}

function renderGitLabSyncSection(id, enabled, freq, err) {
    var freqOptions = ['daily', 'weekly', 'monthly'].map(function(f) {
        return '<option value="' + f + '"' + (f === freq ? ' selected' : '') + '>' +
            f.charAt(0).toUpperCase() + f.slice(1) + '</option>';
    }).join('');
    var errHtml = err ? '<div style="color:#f87171;font-size:0.75rem;margin-top:0.3rem">' + esc(err) + '</div>' : '';
    if (!enabled) {
        return '<div class="webhook-add" style="margin-top:0">' +
                '<select id="gl-freq-' + esc(id) + '" class="webhook-input">' + freqOptions + '</select>' +
                '<button class="btn btn-primary" style="font-size:0.78rem" onclick="doEnableGitLabSync(\'' + esc(id) + '\')">Enable Sync</button>' +
            '</div>' + errHtml;
    }
    return '<div class="toggle-row">' +
            '<span>Syncing ' + esc(freq) + '</span>' +
            '<button class="btn btn-ghost" style="font-size:0.75rem" onclick="doSyncGitLabNow(\'' + esc(id) + '\')">Sync now</button>' +
        '</div>' +
        '<button class="btn btn-outline" style="font-size:0.75rem;margin-top:0.4rem" onclick="doDisableGitLabSync(\'' + esc(id) + '\')">Disable</button>' +
        errHtml;
}

function doEnableGitLabSync(id) {
    var freq = document.getElementById('gl-freq-' + id).value;
    fetch('/api/project-gitlab-sync', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({repo_id: id, enabled: true, frequency: freq})
    }).then(function(r) {
        if (!r.ok) return r.text().then(function(t) { throw new Error(t.trim()); });
        toast('GitLab sync enabled', 'ok');
        loadReleases();
    }).catch(function(err) { toast('Error: ' + err, 'err'); });
}

function doDisableGitLabSync(id) {
    fetch('/api/project-gitlab-sync', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({repo_id: id, enabled: false})
    }).then(function(r) {
        if (!r.ok) return r.text().then(function(t) { throw new Error(t.trim()); });
        toast('GitLab sync disabled', 'ok');
        loadReleases();
    }).catch(function(err) { toast('Error: ' + err, 'err'); });
}

function doSyncGitLabNow(id) {
    fetch('/api/project-gitlab-sync/sync-now?repo_id=' + encodeURIComponent(id), {method: 'POST'})
        .then(function(r) {
            if (!r.ok) return r.text().then(function(t) { throw new Error(t.trim()); });
            toast('Sync queued', 'inf');
            setTimeout(function() { loadReleases(); }, 2000);
        }).catch(function(err) { toast('Error: ' + err, 'err'); });
}

function addEmailToggleToPanel(id, emailImmediate) {
    var panelBody = document.getElementById('settings-panel-body');

    var section = document.createElement('div');
    section.className = 'settings-section';

    var sectionTitle = document.createElement('div');
    sectionTitle.className = 'settings-section-title';
    sectionTitle.textContent = 'Email Notifications';
    section.appendChild(sectionTitle);

    var row = document.createElement('div');
    row.className = 'toggle-row';

    var labelSpan = document.createElement('span');
    labelSpan.textContent = 'Notify on new releases';
    row.appendChild(labelSpan);

    var label = document.createElement('label');
    label.className = 'toggle';

    var cb = document.createElement('input');
    cb.type = 'checkbox';
    cb.id = 'ei-cb-' + id;
    cb.checked = emailImmediate;

    var smtpOk = currentUser && currentUser.smtp_enabled;
    var verified = currentUser && currentUser.email_verified;

    if (!smtpOk) {
        cb.disabled = true;
        cb.title = 'Email not configured';
    } else if (!verified) {
        cb.disabled = true;
        cb.title = 'Verify your email to enable';
    } else {
        cb.addEventListener('change', function() {
            doToggleEmailImmediate(id, cb.checked);
        });
    }

    var slider = document.createElement('span');
    slider.className = 'toggle-slider';
    label.appendChild(cb);
    label.appendChild(slider);
    row.appendChild(label);
    section.appendChild(row);

    panelBody.appendChild(section);
}

function doTogglePush(checkbox, id) {
    var eiCb = document.getElementById('ei-cb-' + id);
    var emailImmediate = eiCb ? eiCb.checked : false;
    fetch('/api/project-settings', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({repo_id: id, push_enabled: checkbox.checked, email_immediate: emailImmediate})
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

function doTogglePrereleases(checkbox, id) {
    var pushCb = document.getElementById('push-cb-' + id);
    var eiCb = document.getElementById('ei-cb-' + id);
    fetch('/api/project-settings', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({
            repo_id: id,
            push_enabled: pushCb ? pushCb.checked : false,
            email_immediate: eiCb ? eiCb.checked : false,
            include_prereleases: checkbox.checked
        })
    }).then(function(r) {
        if (!r.ok) throw new Error('HTTP ' + r.status);
        toast(checkbox.checked ? 'Prereleases included.' : 'Prereleases hidden.', 'ok');
        loadReleases();
    }).catch(function() {
        checkbox.checked = !checkbox.checked;
        toast('Failed to update setting.', 'err');
    });
}

function doToggleEmailImmediate(id, checked) {
    var pushCb = document.getElementById('push-cb-' + id);
    var pushEnabled = pushCb ? pushCb.checked : false;
    fetch('/api/project-settings', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({repo_id: id, push_enabled: pushEnabled, email_immediate: checked})
    }).then(function(r) {
        if (r.ok) {
            toast('Notification setting updated.', 'ok');
        } else {
            var cb = document.getElementById('ei-cb-' + id);
            if (cb) cb.checked = !checked;
            toast('Failed to update setting.', 'err');
        }
    }).catch(function() {
        var cb = document.getElementById('ei-cb-' + id);
        if (cb) cb.checked = !checked;
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
        var backupUrl = p.gitlab_project_path ? p.gitlab_project_path.replace(/\.git$/, '') : '';
        var backupLink = (p.gitlab_sync_enabled && /^https?:\/\//i.test(backupUrl)) ?
            '<a href="' + esc(backupUrl) + '" target="_blank" rel="noopener noreferrer" class="badge">[BACKUP]</a>' : '';
        return '<div class="card" id="projcard-' + esc(p.id) + '">' +
            '<div class="proj-row">' +
                '<div class="proj-card-info">' +
                    '<div class="proj-card-name">' + esc(p.name) + '</div>' +
                    '<div class="proj-card-meta">' +
                        '<span class="badge ' + esc(p.platform) + '">' + esc(p.platform) + '</span>' +
                        backupLink +
                        '<span style="color:#8796ad;font-size:0.75rem">' + sub + '</span>' +
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
    panel.innerHTML = '<div style="font-size:0.8rem;color:#8796ad;padding:0.25rem 0">Loading...</div>';
    fetch('/api/webhooks?repo_id=' + encodeURIComponent(repoID))
        .then(apiJSON)
        .then(function(hooks) { panel.innerHTML = renderWebhookPanel(repoID, hooks || []); })
        .catch(function() { panel.innerHTML = '<div style="color:#f87171;font-size:0.8rem">Failed to load webhooks.</div>'; });
}

function renderWebhookPanel(repoID, hooks) {
    var list = hooks.length ? hooks.map(function(h) {
        var label = h.has_secret ? ' <span style="color:#8796ad;font-size:0.7rem">[signed]</span>' : '';
        return '<div class="webhook-item">' +
            '<span class="webhook-url">' + esc(h.url) + label + '</span>' +
            '<button class="btn btn-ghost" style="color:#f87171;font-size:0.75rem;padding:0.2rem 0.5rem" ' +
                'data-wid="' + esc(h.id) + '" data-repo="' + esc(repoID) + '" onclick="doDeleteWebhook(this)">Remove</button>' +
        '</div>';
    }).join('') : '<div style="font-size:0.8rem;color:#8796ad;padding:0.25rem 0 0.5rem">No webhooks configured.</div>';

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
            if (!r.ok) throw new Error('failed');
            loadWebhooks(repoID); toast('Webhook removed.');
        }).catch(function() { toast('Could not remove webhook.', 'err'); });
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
    'helm-artifacthub': {
        label: 'Artifact Hub URL',
        placeholder: 'https://artifacthub.io/packages/helm/bitnami/redis',
        hint: 'Paste the full Artifact Hub chart URL'
    },
    'helm-repo': {
        label: 'Repo base URL',
        placeholder: 'https://charts.bitnami.com/bitnami',
        hint: 'Base URL of the Helm repo — enter the chart name in the Name field above'
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
    if (/github\.com\//.test(url))                         return 'github';
    if (/gitlab\.com\//.test(url))                         return 'gitlab';
    if (/npmjs\.com\/package\//.test(url))                 return 'npm';
    if (/pypi\.org\/project\//.test(url))                  return 'pypi';
    if (/hub\.docker\.com\/r\//.test(url))                 return 'docker';
    if (/artifacthub\.io\/packages\/helm\//.test(url))     return 'helm-artifacthub';
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
        case 'helm-artifacthub':
            return url.replace(/^.*artifacthub\.io\/packages\/helm\/[^/]+\//, '').split('/')[0];
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
    } else if (platform === 'helm-artifacthub') {
        name = extractNameFromURL(platform, val);
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
            toast('URL not recognized. Supported: GitHub, GitLab, npmjs.com, pypi.org, hub.docker.com, artifacthub.io', 'err');
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

document.getElementById('detail-dialog').addEventListener('click', function(e) {
    if (e.target === this) this.close(); // backdrop click
});
