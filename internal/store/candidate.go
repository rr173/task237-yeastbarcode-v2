package store

import (
	"database/sql"
	"fmt"
	"time"

	"task237-yeastbarcode/internal/model"
)

// SaveCandidate inserts a contamination candidate.
func (s *Store) SaveCandidate(c *model.ContaminationCandidate) error {
	decided := ""
	if c.DecidedAt != nil {
		decided = c.DecidedAt.Format(time.RFC3339)
	}
	_, err := s.db.Exec(
		`INSERT INTO candidates (id, lineage_id, generation, barcode, evidence_score, frequency, source, status, verdict_note, decided_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET evidence_score=excluded.evidence_score, frequency=excluded.frequency, source=excluded.source, status=excluded.status, verdict_note=excluded.verdict_note, decided_at=excluded.decided_at`,
		c.ID, c.LineageID, c.Generation, c.Barcode, c.EvidenceScore, c.Frequency, c.Source, string(c.Status), c.VerdictNote, decided)
	if err != nil {
		return fmt.Errorf("store: save candidate: %w", err)
	}
	return nil
}

// UpdateCandidateStatus transitions a candidate to a new status.
func (s *Store) UpdateCandidateStatus(id string, status model.CandidateStatus, note string) error {
	_, err := s.db.Exec(
		`UPDATE candidates SET status = ?, verdict_note = ? WHERE id = ?`,
		string(status), note, id)
	if err != nil {
		return fmt.Errorf("store: update candidate: %w", err)
	}
	return nil
}

// GetCandidate fetches one contamination candidate by id.
func (s *Store) GetCandidate(id string) (*model.ContaminationCandidate, error) {
	row := s.db.QueryRow(
		`SELECT id, lineage_id, generation, barcode, evidence_score, frequency, source, status, verdict_note, decided_at
		 FROM candidates WHERE id = ?`, id)
	return scanCandidate(row)
}

// ListCandidates returns candidates for a lineage ordered by generation, score.
func (s *Store) ListCandidates(lineageID string) ([]*model.ContaminationCandidate, error) {
	rows, err := s.db.Query(
		`SELECT id, lineage_id, generation, barcode, evidence_score, frequency, source, status, verdict_note, decided_at
		 FROM candidates WHERE lineage_id = ? ORDER BY generation, evidence_score DESC`, lineageID)
	if err != nil {
		return nil, fmt.Errorf("store: list candidates: %w", err)
	}
	defer rows.Close()
	out := []*model.ContaminationCandidate{}
	for rows.Next() {
		c, err := scanCandidate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func scanCandidate(sc scanner) (*model.ContaminationCandidate, error) {
	var id, lid, bc, src, status, note, decided string
	var gen int
	var score, freq float64
	if err := sc.Scan(&id, &lid, &gen, &bc, &score, &freq, &src, &status, &note, &decided); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("store: scan candidate: %w", err)
	}
	c := &model.ContaminationCandidate{
		ID: id, LineageID: lid, Generation: gen, Barcode: bc,
		EvidenceScore: score, Frequency: freq, Source: src,
		Status: model.CandidateStatus(status), VerdictNote: note,
	}
	if decided != "" {
		if t, err := time.Parse(time.RFC3339, decided); err == nil {
			c.DecidedAt = &t
		}
	}
	return c, nil
}
