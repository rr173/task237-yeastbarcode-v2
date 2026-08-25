package store

import (
	"database/sql"
	"fmt"

	"task237-yeastbarcode/internal/model"
)

// SaveCluster inserts or replaces a correction cluster.
func (s *Store) SaveCluster(c *model.CorrectionCluster) error {
	_, err := s.db.Exec(
		`INSERT INTO correction_clusters (id, lineage_id, generation, canonical_barcode, read_count, is_ancestor)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET canonical_barcode=excluded.canonical_barcode, read_count=excluded.read_count, is_ancestor=excluded.is_ancestor`,
		c.ID, c.LineageID, c.Generation, c.CanonicalBarcode, c.ReadCount, boolToInt(c.IsAncestor))
	if err != nil {
		return fmt.Errorf("store: save cluster: %w", err)
	}
	return nil
}

// DeleteClustersForGeneration removes all clusters of a lineage generation so a
// re-correction starts from a clean slate.
func (s *Store) DeleteClustersForGeneration(lineageID string, generation int) error {
	if _, err := s.db.Exec(
		`DELETE FROM correction_clusters WHERE lineage_id = ? AND generation = ?`, lineageID, generation); err != nil {
		return fmt.Errorf("store: delete clusters: %w", err)
	}
	return nil
}

// ListClusters returns clusters for a lineage ordered by generation then count.
func (s *Store) ListClusters(lineageID string) ([]*model.CorrectionCluster, error) {
	rows, err := s.db.Query(
		`SELECT id, lineage_id, generation, canonical_barcode, read_count, is_ancestor
		 FROM correction_clusters WHERE lineage_id = ? ORDER BY generation, read_count DESC`, lineageID)
	if err != nil {
		return nil, fmt.Errorf("store: list clusters: %w", err)
	}
	defer rows.Close()
	out := []*model.CorrectionCluster{}
	for rows.Next() {
		var c model.CorrectionCluster
		var anc int
		if err := rows.Scan(&c.ID, &c.LineageID, &c.Generation, &c.CanonicalBarcode, &c.ReadCount, &anc); err != nil {
			return nil, fmt.Errorf("store: scan cluster: %w", err)
		}
		c.IsAncestor = anc != 0
		out = append(out, &c)
	}
	return out, rows.Err()
}

// ListAncestorBarcodes returns the persisted ancestor barcode set for a lineage.
func (s *Store) ListAncestorBarcodes(lineageID string) (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT canonical_barcode FROM correction_clusters WHERE lineage_id = ? AND is_ancestor = 1`, lineageID)
	if err != nil {
		return nil, fmt.Errorf("store: list ancestor barcodes: %w", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var barcode string
		if err := rows.Scan(&barcode); err != nil {
			return nil, fmt.Errorf("store: scan ancestor barcode: %w", err)
		}
		out[barcode] = true
	}
	return out, rows.Err()
}

// AncestorGeneration returns the locked ancestor generation, if one exists.
func (s *Store) AncestorGeneration(lineageID string) (int, bool, error) {
	var generation int
	err := s.db.QueryRow(`SELECT generation FROM correction_clusters WHERE lineage_id = ? AND is_ancestor = 1 ORDER BY generation LIMIT 1`, lineageID).Scan(&generation)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("store: ancestor generation: %w", err)
	}
	return generation, true, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
