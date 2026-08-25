package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"task237-yeastbarcode/internal/model"

	_ "modernc.org/sqlite"
)

// SaveSnapshot inserts a new discrimination snapshot. It never overwrites an
// existing snapshot: an id collision (which would let a later publish rewrite an
// already-published snapshot's summary and frozen result) is surfaced as
// model.ErrDuplicate rather than silently applied.
func (s *Store) SaveSnapshot(snap *model.DiscriminationSnapshot) error {
	_, err := s.db.Exec(
		`INSERT INTO snapshots (id, lineage_id, status, summary, result_json, created_at, superseded_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		snap.ID, snap.LineageID, string(snap.Status), snap.Summary, snap.ResultJSON, snap.CreatedAt.Format(time.RFC3339Nano), snap.SupersededBy)
	if err != nil {
		if isUniqueViolation(err) {
			return model.ErrDuplicate
		}
		return fmt.Errorf("store: save snapshot: %w", err)
	}
	return nil
}

// isUniqueViolation reports whether err is a SQLite UNIQUE-constraint failure.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// SupersedeSnapshot marks a snapshot as superseded by another.
func (s *Store) SupersedeSnapshot(oldID, newID string) error {
	_, err := s.db.Exec(`UPDATE snapshots SET status = ?, superseded_by = ? WHERE id = ?`,
		string(model.SnapSuperseded), newID, oldID)
	if err != nil {
		return fmt.Errorf("store: supersede snapshot: %w", err)
	}
	return nil
}

// PublishSnapshotIfDraft atomically publishes a draft snapshot once.
func (s *Store) PublishSnapshotIfDraft(id string) (bool, error) {
	result, err := s.db.Exec(
		`UPDATE snapshots SET status = ? WHERE id = ? AND status = ?`,
		string(model.SnapPublished), id, string(model.SnapDraft))
	if err != nil {
		return false, fmt.Errorf("store: publish snapshot: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("store: inspect snapshot publication: %w", err)
	}
	return rows == 1, nil
}

// SupersedeOtherSnapshots closes older drafts and publications when one result is confirmed.
func (s *Store) SupersedeOtherSnapshots(lineageID, keepID string) error {
	_, err := s.db.Exec(
		`UPDATE snapshots SET status = ?, superseded_by = ?
		 WHERE lineage_id = ? AND id <> ? AND status = ?`,
		string(model.SnapSuperseded), keepID, lineageID, keepID, string(model.SnapPublished))
	if err != nil {
		return fmt.Errorf("store: supersede other snapshots: %w", err)
	}
	return nil
}

// HasNewerSnapshot reports whether a later draft or publication exists for the lineage.
func (s *Store) HasNewerSnapshot(lineageID, id string, createdAt time.Time) (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM snapshots WHERE lineage_id = ? AND id <> ? AND created_at > ?`, lineageID, id, createdAt.Format(time.RFC3339Nano)).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("store: check newer snapshot: %w", err)
	}
	return count > 0, nil
}

// GetSnapshot fetches a snapshot by id.
func (s *Store) GetSnapshot(id string) (*model.DiscriminationSnapshot, error) {
	row := s.db.QueryRow(
		`SELECT id, lineage_id, status, summary, result_json, created_at, superseded_by FROM snapshots WHERE id = ?`, id)
	return scanSnapshot(row)
}

// ListSnapshots returns snapshots for a lineage ordered by creation time.
func (s *Store) ListSnapshots(lineageID string) ([]*model.DiscriminationSnapshot, error) {
	rows, err := s.db.Query(
		`SELECT id, lineage_id, status, summary, result_json, created_at, superseded_by FROM snapshots WHERE lineage_id = ? ORDER BY created_at`, lineageID)
	if err != nil {
		return nil, fmt.Errorf("store: list snapshots: %w", err)
	}
	defer rows.Close()
	out := []*model.DiscriminationSnapshot{}
	for rows.Next() {
		snap, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, snap)
	}
	return out, rows.Err()
}

func scanSnapshot(sc scanner) (*model.DiscriminationSnapshot, error) {
	var id, lid, status, summary, result, created, sup string
	if err := sc.Scan(&id, &lid, &status, &summary, &result, &created, &sup); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("store: scan snapshot: %w", err)
	}
	snap := &model.DiscriminationSnapshot{
		ID: id, LineageID: lid, Status: model.SnapshotStatus(status),
		Summary: summary, ResultJSON: result, SupersededBy: sup,
	}
	if t, err := time.Parse(time.RFC3339, created); err == nil {
		snap.CreatedAt = t
	}
	return snap, nil
}
