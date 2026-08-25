package store

import (
	"database/sql"
	"fmt"
	"time"

	"task237-yeastbarcode/internal/model"
)

// SaveLineage inserts or replaces a culture lineage.
func (s *Store) SaveLineage(l *model.CultureLineage) error {
	sealed := ""
	if l.SealedAt != nil {
		sealed = l.SealedAt.Format(time.RFC3339)
	}
	_, err := s.db.Exec(
		`INSERT INTO lineages (id, name, status, created_at, sealed_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET name=excluded.name, status=excluded.status, sealed_at=excluded.sealed_at`,
		l.ID, l.Name, string(l.Status), l.CreatedAt.Format(time.RFC3339), sealed)
	if err != nil {
		return fmt.Errorf("store: save lineage: %w", err)
	}
	return nil
}

// GetLineage fetches a lineage by ID.
func (s *Store) GetLineage(id string) (*model.CultureLineage, error) {
	row := s.db.QueryRow(`SELECT id, name, status, created_at, sealed_at FROM lineages WHERE id = ?`, id)
	return scanLineage(row)
}

// EnsureLineageMutable rejects writes after a lineage has been sealed.
func (s *Store) EnsureLineageMutable(id string) error {
	lineage, err := s.GetLineage(id)
	if err != nil {
		return err
	}
	if lineage.Status == model.LineageSealed {
		return model.ErrSealedMutate
	}
	return nil
}

// ListLineages returns all lineages ordered by creation time.
func (s *Store) ListLineages() ([]*model.CultureLineage, error) {
	rows, err := s.db.Query(`SELECT id, name, status, created_at, sealed_at FROM lineages ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("store: list lineages: %w", err)
	}
	defer rows.Close()
	out := []*model.CultureLineage{}
	for rows.Next() {
		l, err := scanLineage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func scanLineage(sc scanner) (*model.CultureLineage, error) {
	var id, name, status, created, sealed string
	if err := sc.Scan(&id, &name, &status, &created, &sealed); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("store: scan lineage: %w", err)
	}
	l := &model.CultureLineage{ID: id, Name: name, Status: model.LineageStatus(status)}
	if t, err := time.Parse(time.RFC3339, created); err == nil {
		l.CreatedAt = t
	}
	if sealed != "" {
		if t, err := time.Parse(time.RFC3339, sealed); err == nil {
			l.SealedAt = &t
		}
	}
	return l, nil
}

// SaveGenerationEdge inserts a parent/child generation edge.
func (s *Store) SaveGenerationEdge(e model.GenerationEdge) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO generation_edges (lineage_id, parent_generation, child_generation) VALUES (?, ?, ?)`,
		e.LineageID, e.ParentGeneration, e.ChildGeneration)
	if err != nil {
		return fmt.Errorf("store: save edge: %w", err)
	}
	return nil
}

// GenerationEdgeExists reports whether an exact edge is already persisted.
func (s *Store) GenerationEdgeExists(e model.GenerationEdge) (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM generation_edges WHERE lineage_id = ? AND parent_generation = ? AND child_generation = ?`, e.LineageID, e.ParentGeneration, e.ChildGeneration).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("store: check edge: %w", err)
	}
	return count != 0, nil
}

// ListGenerationEdges returns edges for a lineage ordered by child generation.
func (s *Store) ListGenerationEdges(lineageID string) ([]model.GenerationEdge, error) {
	rows, err := s.db.Query(
		`SELECT lineage_id, parent_generation, child_generation FROM generation_edges WHERE lineage_id = ? ORDER BY child_generation`,
		lineageID)
	if err != nil {
		return nil, fmt.Errorf("store: list edges: %w", err)
	}
	defer rows.Close()
	out := []model.GenerationEdge{}
	for rows.Next() {
		var e model.GenerationEdge
		if err := rows.Scan(&e.LineageID, &e.ParentGeneration, &e.ChildGeneration); err != nil {
			return nil, fmt.Errorf("store: scan edge: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}
