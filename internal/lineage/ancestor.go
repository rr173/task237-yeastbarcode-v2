package lineage

import (
	"fmt"

	"task237-yeastbarcode/internal/model"
)

// LockAncestor marks the founding-generation cluster(s) as the trusted ancestor
// clone barcode(s). Locked ancestors are the reference against which later
// generations are compared; a barcode appearing in a descendant that is NOT in
// the ancestor set and not explainable by inheritance is a contamination signal.
func (s *Service) LockAncestor(lineageID string, generation int) error {
	lockedGeneration, locked, err := s.st.AncestorGeneration(lineageID)
	if err != nil {
		return err
	}
	if locked && lockedGeneration != generation {
		return fmt.Errorf("lineage: ancestor already locked at generation %d", lockedGeneration)
	}
	clusters, err := s.st.ListClusters(lineageID)
	if err != nil {
		return err
	}
	found := false
	for _, c := range clusters {
		if c.Generation == generation {
			c.IsAncestor = true
			if err := s.st.SaveCluster(c); err != nil {
				return err
			}
			found = true
		}
	}
	if !found {
		return model.ErrNotFound
	}
	return nil
}

// AncestorBarcodes returns the set of locked ancestor canonical barcodes.
func (s *Service) AncestorBarcodes(lineageID string) (map[string]bool, error) {
	return s.st.ListAncestorBarcodes(lineageID)
}

// RestoreAncestorLocks reapplies persisted ancestor decisions after a cluster rebuild.
func (s *Service) RestoreAncestorLocks(lineageID string, locked map[string]bool, clusters []*model.CorrectionCluster) error {
	for _, cluster := range clusters {
		if locked[cluster.CanonicalBarcode] {
			cluster.IsAncestor = true
			if err := s.st.SaveCluster(cluster); err != nil {
				return err
			}
		}
	}
	return nil
}
