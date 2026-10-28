package main

import (
	"database/sql"
	"testing"
)

func testMemDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1) // same as NewStore; also keeps :memory: on one connection
	db.Exec("PRAGMA foreign_keys=ON")
	t.Cleanup(func() { db.Close() })
	return db
}

func upTo(version int) []migration {
	var out []migration
	for _, m := range migrations {
		if m.version <= version {
			out = append(out, m)
		}
	}
	return out
}

func userVersion(t *testing.T, db *sql.DB) int {
	t.Helper()
	var v int
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestMigrationsAreOrdered(t *testing.T) {
	for i := 1; i < len(migrations); i++ {
		if migrations[i].version != migrations[i-1].version+1 {
			t.Fatalf("migration %d follows %d; versions must be consecutive", migrations[i].version, migrations[i-1].version)
		}
	}
}

func TestMigrateFreshAndIdempotent(t *testing.T) {
	db := testMemDB(t)
	latest := migrations[len(migrations)-1].version
	for i := 0; i < 2; i++ { // second run must be a no-op
		if err := migrate(db); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
		if v := userVersion(t, db); v != latest {
			t.Fatalf("run %d: user_version = %d, want %d", i+1, v, latest)
		}
	}
}

func TestMigrateV10KeepsUsersAndForeignKeys(t *testing.T) {
	db := testMemDB(t)
	if err := runMigrations(db, upTo(9)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users (id, username, password_hash, email, email_verified, page_size)
		VALUES ('u1', 'old-name', 'hash', 'a@example.com', 1, 20)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sessions (id, user_id, expires_at) VALUES ('s1', 'u1', '2099-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}

	if err := migrate(db); err != nil {
		t.Fatal(err)
	}

	var email string
	var verified, pageSize int
	if err := db.QueryRow("SELECT email, email_verified, page_size FROM users WHERE id = 'u1'").Scan(&email, &verified, &pageSize); err != nil {
		t.Fatalf("user lost in v10 rebuild: %v", err)
	}
	if email != "a@example.com" || verified != 1 || pageSize != 20 {
		t.Fatalf("user changed: %q %d %d", email, verified, pageSize)
	}
	var sessions int
	db.QueryRow("SELECT COUNT(*) FROM sessions WHERE user_id = 'u1'").Scan(&sessions)
	if sessions != 1 {
		t.Fatalf("session cascaded away during rebuild: %d", sessions)
	}
	if rows, err := db.Query("PRAGMA foreign_key_check"); err != nil {
		t.Fatal(err)
	} else {
		defer rows.Close()
		if rows.Next() {
			t.Fatal("foreign_key_check reports violations after migration")
		}
	}
	var fk int
	db.QueryRow("PRAGMA foreign_keys").Scan(&fk)
	if fk != 1 {
		t.Fatal("foreign_keys was not re-enabled after the v10 rebuild")
	}
}

func TestFailedMigrationRollsBack(t *testing.T) {
	db := testMemDB(t)
	if err := runMigrations(db, upTo(13)); err != nil {
		t.Fatal(err)
	}
	bad := append(upTo(13), migration{version: 14, stmts: []string{
		`ALTER TABLE releases ADD COLUMN half_applied INTEGER`,
		`THIS IS NOT SQL`,
	}})
	if err := runMigrations(db, bad); err == nil {
		t.Fatal("expected error from bad migration")
	}
	if v := userVersion(t, db); v != 13 {
		t.Fatalf("user_version = %d after failed v14, want 13", v)
	}
	rows, err := db.Query("PRAGMA table_info(releases)")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
		if name == "half_applied" {
			t.Fatal("first statement of the failed migration was not rolled back")
		}
	}
}
