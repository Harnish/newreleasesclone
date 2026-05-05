# Helm Chart Support Design

**Date:** 2026-05-05
**Status:** Approved

## Summary

Add Helm chart release tracking to newreleases, supporting two sources:
- **Artifact Hub** (`helm-artifacthub`) — public charts via `artifacthub.io` REST API
- **Direct Helm repo** (`helm-repo`) — any Helm repo via its `index.yaml`

Both display version as `<chart_version> (app <app_version>)`.

---

## Architecture

### New Platform Identifiers

| Platform | Source | User provides |
|---|---|---|
| `helm-artifacthub` | Artifact Hub REST API | Artifact Hub URL (e.g., `https://artifacthub.io/packages/helm/bitnami/redis`) |
| `helm-repo` | Raw `index.yaml` download | Repo base URL in URL field; chart name in Name field |

No schema changes — both platforms fit the existing `repos`/`releases` tables unchanged.

### Version Format

Both platforms store version as: `17.0.0 (app 7.2.1)`

This is encoded directly into the single `version` TEXT column — no additional columns needed.

### Release IDs

- Artifact Hub: `helm_ah_{chart_version}` (e.g., `helm_ah_17.0.0`)
- Helm repo: `helm_repo_{chart_version}` (e.g., `helm_repo_17.0.0`)

---

## Fetchers (`fetchers.go`)

### `fetchHelmArtifactHub(project Project) []Release`

1. Parse `project.RepoURL` to extract `{repoName}/{packageName}` from an Artifact Hub URL
   - Pattern: `https://artifacthub.io/packages/helm/{repoName}/{packageName}`
2. Call `GET https://artifacthub.io/api/v1/packages/helm/{repoName}/{packageName}`
   - Returns `available_versions` array: `[{version, ts, prerelease}, ...]`
3. Filter out prereleases. If none remain, fall back to including all non-draft versions.
4. Take up to 10 versions.
5. For each, call `GET https://artifacthub.io/api/v1/packages/helm/{repoName}/{packageName}/{version}` to retrieve `app_version`.
   - If an individual version call fails, skip that version and continue.
6. Build `Release` with:
   - `ID`: `helm_ah_{version}`
   - `Version`: `{version} (app {app_version})`
   - `URL`: original Artifact Hub URL
   - `PublishedAt`: from `ts` (Unix timestamp)
   - `Description`: package description (truncated to 200 chars)

Max ~11 HTTP calls per refresh (1 list + up to 10 version details). All refreshes run in background goroutines, so this is acceptable.

### `fetchHelmRepo(project Project) []Release`

1. Fetch `{project.RepoURL}/index.yaml`
2. Parse YAML using `gopkg.in/yaml.v3` into the Helm index structure:
   ```
   apiVersion: v1
   entries:
     {chartName}:
       - version: "17.0.0"
         appVersion: "7.2.1"
         created: "2023-01-01T00:00:00Z"
         urls: [...]
   ```
3. Look up `entries[project.Name]`. If not found, log warning and return nil.
4. Sort by `created` descending.
5. Filter versions containing `-` (semver prerelease convention). Fallback to all if none remain.
6. Take up to 10 versions.
7. Build `Release` with:
   - `ID`: `helm_repo_{version}`
   - `Version`: `{version} (app {appVersion})`
   - `URL`: first entry in `urls[]` if present, else repo base URL
   - `PublishedAt`: from `created` field
   - `Description`: `"Helm Chart"`

---

## Store (`store.go`)

Add two cases to the `RefreshProject` switch:

```go
case "helm-artifacthub":
    releases = fetchHelmArtifactHub(project)
case "helm-repo":
    releases = fetchHelmRepo(project)
```

---

## UI (`ui.go`)

### Platform dropdown (2 new options)

```html
<option value="helm-artifacthub">Helm (Artifact Hub)</option>
<option value="helm-repo">Helm (Repo)</option>
```

### Badge CSS (2 new classes)

```css
.badge.helm-artifacthub { background: #1a2e1a; color: #4ade80; }
.badge.helm-repo        { background: #1a2e1a; color: #86efac; }
```

### `platformConfig` entries

```js
'helm-artifacthub': {
    label: 'Artifact Hub URL',
    placeholder: 'https://artifacthub.io/packages/helm/bitnami/redis',
    hint: 'Paste the full Artifact Hub chart URL'
},
'helm-repo': {
    label: 'Repo URL',
    placeholder: 'https://charts.bitnami.com/bitnami',
    hint: 'Base URL of the Helm repo; enter chart name in the Name field'
},
```

### `detectPlatform()` — new pattern

```js
if (/artifacthub\.io\/packages\/helm\//.test(url)) return 'helm-artifacthub';
```

(No auto-detection for `helm-repo` — raw repo URLs have no distinguishing pattern.)

### `extractNameFromURL()` — new case

```js
case 'helm-artifacthub':
    return url.replace(/^.*artifacthub\.io\/packages\/helm\/[^/]+\//, '').split('/')[0];
```

---

## Dependencies

Add `gopkg.in/yaml.v3` to `go.mod` for `index.yaml` parsing (only needed by `fetchHelmRepo`).

---

## Error Handling

All fetchers follow existing conventions:
- HTTP errors or non-200 responses → `log.Printf` warning + return `nil`
- Parse failures → `log.Printf` + return `nil`
- Artifact Hub individual version call failure → skip that version, continue
- `helm-repo` chart name not found in index → log warning + return `nil`

---

## Testing

Two new test functions following existing patterns (mock HTTP server, in-memory SQLite):

- **`TestFetchHelmArtifactHub`** — mock server returns sample Artifact Hub JSON; asserts correct version string format, prerelease filtering, release count ≤ 10
- **`TestFetchHelmRepo`** — mock server returns a minimal `index.yaml`; asserts correct version string, prerelease filtering, release count ≤ 10

---

## Files Changed

| File | Change |
|---|---|
| `fetchers.go` | Add `fetchHelmArtifactHub` and `fetchHelmRepo` |
| `store.go` | Add 2 cases to `RefreshProject` switch |
| `ui.go` | Dropdown options, badge CSS, platformConfig, detectPlatform, extractNameFromURL |
| `go.mod` / `go.sum` | Add `gopkg.in/yaml.v3` |
| `fetchers_test.go` (new or existing) | Add `TestFetchHelmArtifactHub`, `TestFetchHelmRepo` |

No changes to `models.go`, `handlers.go`, or `main.go`.
