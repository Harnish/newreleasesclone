package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

var store *Store

const releaseLimit = 50

type Store struct {
	db *sql.DB
}

func NewStore(dbPath string) (*Store, error) {
	if dbPath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create data directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Serialize writes through a single connection; WAL allows concurrent reads
	db.SetMaxOpenConns(1)

	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			return nil, fmt.Errorf("failed to apply %s: %w", pragma, err)
		}
	}

	if err := migrate(db); err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS projects (
			id            TEXT PRIMARY KEY,
			name          TEXT NOT NULL,
			platform      TEXT NOT NULL,
			repo_url      TEXT NOT NULL,
			last_refresh  TEXT NOT NULL DEFAULT '',
			refresh_count INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS releases (
			id            TEXT NOT NULL,
			project_id    TEXT NOT NULL,
			name          TEXT NOT NULL,
			version       TEXT NOT NULL,
			platform      TEXT NOT NULL,
			url           TEXT NOT NULL,
			published_at  TEXT NOT NULL DEFAULT '',
			description   TEXT NOT NULL DEFAULT '',
			release_notes TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (id, project_id)
		);
	`)
	return err
}

func (s *Store) AddProject(p Project) {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO projects (id, name, platform, repo_url, last_refresh, refresh_count)
		VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Platform, p.RepoURL,
		p.LastRefresh.Format(time.RFC3339), p.RefreshCount,
	)
	if err != nil {
		log.Printf("⚠ Failed to add project %s: %v", p.ID, err)
	}
}

func (s *Store) DeleteProject(projectID string) bool {
	tx, err := s.db.Begin()
	if err != nil {
		log.Printf("⚠ Failed to begin delete transaction for %s: %v", projectID, err)
		return false
	}
	tx.Exec("DELETE FROM releases WHERE project_id = ?", projectID)
	res, err := tx.Exec("DELETE FROM projects WHERE id = ?", projectID)
	if err != nil {
		tx.Rollback()
		log.Printf("⚠ Failed to delete project %s: %v", projectID, err)
		return false
	}
	tx.Commit()
	n, _ := res.RowsAffected()
	return n > 0
}

func (s *Store) GetProjects() []Project {
	rows, err := s.db.Query(`
		SELECT id, name, platform, repo_url, last_refresh, refresh_count FROM projects`)
	if err != nil {
		log.Printf("⚠ Failed to query projects: %v", err)
		return []Project{}
	}
	defer rows.Close()

	projects := []Project{}
	for rows.Next() {
		var p Project
		var lastRefresh string
		if err := rows.Scan(&p.ID, &p.Name, &p.Platform, &p.RepoURL, &lastRefresh, &p.RefreshCount); err != nil {
			log.Printf("⚠ Failed to scan project: %v", err)
			continue
		}
		if lastRefresh != "" {
			p.LastRefresh, _ = time.Parse(time.RFC3339, lastRefresh)
		}
		projects = append(projects, p)
	}
	return projects
}

func (s *Store) AddRelease(projectID string, r Release) {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO releases
			(id, project_id, name, version, platform, url, published_at, description, release_notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, projectID, r.Name, r.Version, r.Platform, r.URL,
		r.PublishedAt.Format(time.RFC3339), r.Description, r.ReleaseNotes,
	)
	if err != nil {
		log.Printf("⚠ Failed to add release %s: %v", r.ID, err)
		return
	}

	// Trim oldest beyond the limit
	_, err = s.db.Exec(`
		DELETE FROM releases
		WHERE project_id = ?
		  AND rowid NOT IN (
		      SELECT rowid FROM releases
		      WHERE project_id = ?
		      ORDER BY published_at DESC
		      LIMIT ?
		  )`, projectID, projectID, releaseLimit)
	if err != nil {
		log.Printf("⚠ Failed to trim releases for %s: %v", projectID, err)
	}
}

func (s *Store) GetReleases(projectID string) []Release {
	rows, err := s.db.Query(`
		SELECT id, project_id, name, version, platform, url, published_at, description, release_notes
		FROM releases
		WHERE project_id = ?
		ORDER BY published_at DESC`, projectID)
	if err != nil {
		log.Printf("⚠ Failed to query releases for %s: %v", projectID, err)
		return nil
	}
	defer rows.Close()
	return scanReleases(rows)
}

func (s *Store) GetAllReleases() []Release {
	// JOIN ensures orphaned releases (from deleted projects) are excluded.
	rows, err := s.db.Query(`
		SELECT r.id, r.project_id, r.name, r.version, r.platform, r.url, r.published_at, r.description, r.release_notes
		FROM releases r
		INNER JOIN projects p ON p.id = r.project_id
		ORDER BY r.published_at DESC`)
	if err != nil {
		log.Printf("⚠ Failed to query all releases: %v", err)
		return []Release{}
	}
	defer rows.Close()
	return scanReleases(rows)
}

func scanReleases(rows *sql.Rows) []Release {
	releases := []Release{}
	for rows.Next() {
		var r Release
		var publishedAt string
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Name, &r.Version, &r.Platform, &r.URL,
			&publishedAt, &r.Description, &r.ReleaseNotes); err != nil {
			log.Printf("⚠ Failed to scan release: %v", err)
			continue
		}
		r.PublishedAt, _ = time.Parse(time.RFC3339, publishedAt)
		releases = append(releases, r)
	}
	return releases
}

func (s *Store) MarkRefreshed(projectID string) {
	_, err := s.db.Exec(`
		UPDATE projects
		SET last_refresh = ?, refresh_count = refresh_count + 1
		WHERE id = ?`,
		time.Now().Format(time.RFC3339), projectID,
	)
	if err != nil {
		log.Printf("⚠ Failed to mark project %s as refreshed: %v", projectID, err)
	}
}

func (s *Store) IsStale(projectID string) bool {
	var lastRefresh string
	err := s.db.QueryRow("SELECT last_refresh FROM projects WHERE id = ?", projectID).Scan(&lastRefresh)
	if err != nil || lastRefresh == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, lastRefresh)
	if err != nil {
		return true
	}
	return time.Since(t) > 30*time.Minute
}

func (s *Store) GetStaleProjects() []Project {
	cutoff := time.Now().Add(-30 * time.Minute).Format(time.RFC3339)
	rows, err := s.db.Query(`
		SELECT id, name, platform, repo_url, last_refresh, refresh_count
		FROM projects
		WHERE last_refresh = '' OR last_refresh < ?`, cutoff)
	if err != nil {
		log.Printf("⚠ Failed to query stale projects: %v", err)
		return []Project{}
	}
	defer rows.Close()

	projects := []Project{}
	for rows.Next() {
		var p Project
		var lastRefresh string
		if err := rows.Scan(&p.ID, &p.Name, &p.Platform, &p.RepoURL, &lastRefresh, &p.RefreshCount); err != nil {
			continue
		}
		if lastRefresh != "" {
			p.LastRefresh, _ = time.Parse(time.RFC3339, lastRefresh)
		}
		projects = append(projects, p)
	}
	return projects
}

func (s *Store) RefreshProject(projectID string) {
	var project Project
	err := s.db.QueryRow(`
		SELECT id, name, platform, repo_url FROM projects WHERE id = ?`, projectID).
		Scan(&project.ID, &project.Name, &project.Platform, &project.RepoURL)
	if err != nil {
		return
	}

	var releases []Release
	switch project.Platform {
	case "github":
		releases = fetchGitHubReleases(project)
	case "npm":
		releases = fetchNPMVersions(project)
	case "pypi":
		releases = fetchPyPIReleases(project)
	case "docker":
		releases = fetchDockerTags(project)
	case "gitlab":
		releases = fetchGitLabReleases(project)
	default:
		log.Printf("⚠ Unknown platform: %s", project.Platform)
		return
	}

	if len(releases) == 0 {
		log.Printf("⚠ No releases found for %s (%s)", project.Name, project.Platform)
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		log.Printf("⚠ Failed to begin transaction for %s: %v", project.Name, err)
		return
	}

	if _, err := tx.Exec("DELETE FROM releases WHERE project_id = ?", projectID); err != nil {
		tx.Rollback()
		log.Printf("⚠ Failed to clear releases for %s: %v", project.Name, err)
		return
	}

	for _, r := range releases {
		_, err := tx.Exec(`
			INSERT INTO releases
				(id, project_id, name, version, platform, url, published_at, description, release_notes)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			r.ID, projectID, r.Name, r.Version, r.Platform, r.URL,
			r.PublishedAt.Format(time.RFC3339), r.Description, r.ReleaseNotes,
		)
		if err != nil {
			tx.Rollback()
			log.Printf("⚠ Failed to insert release for %s: %v", project.Name, err)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("⚠ Failed to commit releases for %s: %v", project.Name, err)
		return
	}

	// Guard: only mark refreshed if the project still exists
	var exists int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM projects WHERE id = ?", projectID).Scan(&exists); err != nil || exists == 0 {
		return
	}
	s.MarkRefreshed(projectID)
	log.Printf("✓ Refreshed %s: found %d releases, latest %s", project.Name, len(releases), releases[0].Version)
}
