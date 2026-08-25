package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"task237-yeastbarcode/internal/model"
)

// SaveRead inserts a barcode read, enforcing the content-hash idempotency key.
// If an identical read already exists, it returns the existing record and
// model.ErrDuplicate so the caller can skip re-processing. The decision is made
// atomically by the INSERT ... ON CONFLICT DO NOTHING verdict, so concurrent
// callers cannot all report a successful insert: exactly the one that wins the
// race returns (r, nil); the rest return (existing, model.ErrDuplicate).
func (s *Store) SaveRead(r *model.BarcodeRead) (*model.BarcodeRead, error) {
	q := strings.TrimSpace(strings.Trim(fmt.Sprint(r.Quality), "[]"))
	res, err := s.db.Exec(
		`INSERT INTO reads (id, lineage_id, generation, raw_barcode, corrected_barcode, quality, status, cluster_id, hash, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(hash) DO NOTHING`,
		r.ID, r.LineageID, r.Generation, r.RawBarcode, r.CorrectedBarcode, q, string(r.Status), r.ClusterID, r.Hash, r.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("store: save read: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		// hash conflict: identical content already stored under another row.
		existing, lerr := s.GetReadByHash(r.Hash)
		if lerr != nil {
			return nil, fmt.Errorf("store: save read: resolve conflict: %w", lerr)
		}
		return existing, model.ErrDuplicate
	}
	return r, nil
}

// GetReadByHash returns a read by its content hash (for idempotent lookup).
func (s *Store) GetReadByHash(hash string) (*model.BarcodeRead, error) {
	row := s.db.QueryRow(
		`SELECT id, lineage_id, generation, raw_barcode, corrected_barcode, quality, status, cluster_id, hash, created_at
		 FROM reads WHERE hash = ?`, hash)
	return scanRead(row)
}

// UpdateReadStatus updates the status and optional corrected barcode of a read.
func (s *Store) UpdateReadStatus(id string, status model.ReadStatus, corrected, clusterID string) error {
	_, err := s.db.Exec(
		`UPDATE reads SET status = ?, corrected_barcode = ?, cluster_id = ? WHERE id = ?`,
		string(status), corrected, clusterID, id)
	if err != nil {
		return fmt.Errorf("store: update read: %w", err)
	}
	return nil
}

// ListReadsByLineage returns reads for a lineage ordered by generation then id.
func (s *Store) ListReadsByLineage(lineageID string) ([]*model.BarcodeRead, error) {
	rows, err := s.db.Query(
		`SELECT id, lineage_id, generation, raw_barcode, corrected_barcode, quality, status, cluster_id, hash, created_at
		 FROM reads WHERE lineage_id = ? ORDER BY generation, id`, lineageID)
	if err != nil {
		return nil, fmt.Errorf("store: list reads: %w", err)
	}
	defer rows.Close()
	out := []*model.BarcodeRead{}
	for rows.Next() {
		r, err := scanRead(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListReadsByGeneration returns reads for a lineage at a specific generation.
func (s *Store) ListReadsByGeneration(lineageID string, generation int) ([]*model.BarcodeRead, error) {
	rows, err := s.db.Query(
		`SELECT id, lineage_id, generation, raw_barcode, corrected_barcode, quality, status, cluster_id, hash, created_at
		 FROM reads WHERE lineage_id = ? AND generation = ? ORDER BY id`, lineageID, generation)
	if err != nil {
		return nil, fmt.Errorf("store: list reads gen: %w", err)
	}
	defer rows.Close()
	out := []*model.BarcodeRead{}
	for rows.Next() {
		r, err := scanRead(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func scanRead(sc scanner) (*model.BarcodeRead, error) {
	var id, lid, raw, corr, q, status, cid, hash, created string
	var gen int
	if err := sc.Scan(&id, &lid, &gen, &raw, &corr, &q, &status, &cid, &hash, &created); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("store: scan read: %w", err)
	}
	r := &model.BarcodeRead{
		ID:               id,
		LineageID:        lid,
		Generation:       gen,
		RawBarcode:       raw,
		CorrectedBarcode: corr,
		Status:           model.ReadStatus(status),
		ClusterID:        cid,
		Hash:             hash,
	}
	if t, err := time.Parse(time.RFC3339, created); err == nil {
		r.CreatedAt = t
	}
	r.Quality = parseQuality(q)
	return r, nil
}

func parseQuality(s string) []int {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, " ")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		var v int
		if _, err := fmt.Sscanf(p, "%d", &v); err == nil {
			out = append(out, v)
		}
	}
	return out
}
