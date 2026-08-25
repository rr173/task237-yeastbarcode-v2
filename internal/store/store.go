// Package store provides SQLite persistence for the yeast barcode
// contamination discriminator. All writes are transactional and the schema
// supports restart recovery of un-finished correction/clustering work.
package store

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// Store wraps a *sql.DB and exposes transactional helpers.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the SQLite database at the given path and runs the
// schema migration. It is safe to call Open on an existing database.
func Open(path string) (*Store, error) {
	dsn := path + "?_journal=WAL&_busy_timeout=5000&_foreign_keys=on"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	db.SetMaxOpenConns(1) // sqlite single-writer; serialized access
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// DB returns the underlying database handle (used by package-specific stores).
func (s *Store) DB() *sql.DB { return s.db }

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// InTx runs fn inside a transaction and commits on success, rolls back on error.
func (s *Store) InTx(fn func(*sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("store: begin tx: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("store: commit tx: %w", err)
	}
	return nil
}

// migrate creates all tables if they do not exist.
func (s *Store) migrate() error {
	schema := strings.Join([]string{
		`CREATE TABLE IF NOT EXISTS lineages (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			sealed_at TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS reads (
			id TEXT PRIMARY KEY,
			lineage_id TEXT NOT NULL,
			generation INTEGER NOT NULL,
			raw_barcode TEXT NOT NULL,
			corrected_barcode TEXT,
			quality TEXT NOT NULL,
			status TEXT NOT NULL,
			cluster_id TEXT,
			hash TEXT NOT NULL UNIQUE,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS generation_edges (
			lineage_id TEXT NOT NULL,
			parent_generation INTEGER NOT NULL,
			child_generation INTEGER NOT NULL,
			PRIMARY KEY (lineage_id, parent_generation, child_generation)
		);`,
		`CREATE TABLE IF NOT EXISTS correction_clusters (
			id TEXT PRIMARY KEY,
			lineage_id TEXT NOT NULL,
			generation INTEGER NOT NULL,
			canonical_barcode TEXT NOT NULL,
			read_count INTEGER NOT NULL,
			is_ancestor INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS candidates (
			id TEXT PRIMARY KEY,
			lineage_id TEXT NOT NULL,
			generation INTEGER NOT NULL,
			barcode TEXT NOT NULL,
			evidence_score REAL NOT NULL,
			frequency REAL NOT NULL,
			source TEXT NOT NULL,
			status TEXT NOT NULL,
			verdict_note TEXT,
			decided_at TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS snapshots (
			id TEXT PRIMARY KEY,
			lineage_id TEXT NOT NULL,
			status TEXT NOT NULL,
			summary TEXT,
			result_json TEXT NOT NULL,
			created_at TEXT NOT NULL,
			superseded_by TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_reads_lineage ON reads(lineage_id, generation);`,
		`CREATE INDEX IF NOT EXISTS idx_clusters_lineage ON correction_clusters(lineage_id, generation);`,
		`CREATE INDEX IF NOT EXISTS idx_candidates_lineage ON candidates(lineage_id, generation);`,
	}, "\n")
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	return nil
}
