# Release Notes Integration

## Overview
Enhanced the release tracking system to include full release notes as part of the release version data structure.

## Changes Made

### 1. Release Structure Update
**File**: `main.go` (lines 19-27)

Added `ReleaseNotes` field to store the complete release notes:
```go
type Release struct {
	ID           string    `json:"id" yaml:"id"`
	Name         string    `json:"name" yaml:"name"`
	Version      string    `json:"version" yaml:"version"`
	Platform     string    `json:"platform" yaml:"platform"`
	URL          string    `json:"url" yaml:"url"`
	PublishedAt  time.Time `json:"published_at" yaml:"published_at"`
	Description  string    `json:"description" yaml:"description"`  // Truncated to 200 chars
	ReleaseNotes string    `json:"release_notes" yaml:"release_notes"` // Full release notes
}
```

### 2. Platform-Specific Updates

#### GitHub Releases (fetchGitHubReleases)
- **Stable releases**: Full `gh.Body` stored in `ReleaseNotes`
- **Prerelease fallback**: Full `gh.Body` stored in `ReleaseNotes`
- Maintains truncated `Description` for display purposes

#### GitLab Releases (fetchGitLabReleases)
- Full `gl.Description` stored in `ReleaseNotes`
- Maintains truncated `Description` for display purposes

#### NPM Versions (fetchNPMVersions)
- `ReleaseNotes`: Empty string (NPM API doesn't provide release notes)
- `Description`: "NPM Package Version"

#### PyPI Releases (fetchPyPIReleases)
- `ReleaseNotes`: Empty string (PyPI API doesn't provide release notes)
- `Description`: "PyPI Package Version"

#### Docker Tags (fetchDockerTags)
- `ReleaseNotes`: Empty string (Docker Hub API doesn't provide release notes)
- `Description`: "Docker Image Tag"

## API Response Changes

All release endpoints now include the `release_notes` field in JSON responses:

```json
{
  "id": "gh_123456",
  "name": "Example Project",
  "version": "v1.2.3",
  "platform": "github",
  "url": "https://github.com/user/repo/releases/tag/v1.2.3",
  "published_at": "2025-11-10T12:00:00Z",
  "description": "Bug fixes and improvements...",
  "release_notes": "## Features\n- New feature X\n- New feature Y\n\n## Bug Fixes\n- Fixed issue A\n- Fixed issue B\n\n## Breaking Changes\n- Removed deprecated API endpoint"
}
```

## State Persistence

The `ReleaseNotes` field is:
- ✅ Persisted to `state.yaml` via YAML serialization
- ✅ Loaded from `state.yaml` on server startup
- ✅ Auto-saved on every release refresh

## Usage

### Frontend JavaScript Access
```javascript
// Access full release notes from API response
const releases = await fetch('/api/releases').then(r => r.json());
releases.forEach(r => {
  console.log(`${r.name} ${r.version}`);
  console.log('Full Release Notes:', r.release_notes);
  console.log('Preview (truncated):', r.description);
});
```

### Programmatic Access
```go
// In Go code, access full notes directly
for _, release := range store.GetReleases(projectID) {
  fmt.Println("Version:", release.Version)
  fmt.Println("Full Notes:", release.ReleaseNotes)
}
```

## Benefits

1. **Complete Information**: Full release notes available for display, analysis, or storage
2. **Backward Compatible**: Existing `Description` field still available for truncated previews
3. **Persistent**: Release notes survive server restarts
4. **API Complete**: JSON API now provides full release information
5. **Platform Support**: Works with platforms that provide release notes (GitHub, GitLab)

## Next Steps (Optional)

- Update frontend HTML template to display full release notes in a modal/expandable section
- Add search/filter capability on full release notes content
- Implement release notes comparison between versions
- Add markdown parsing/rendering for GitHub and GitLab release notes
