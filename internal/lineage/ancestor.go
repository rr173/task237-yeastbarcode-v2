package lineage

import "task237-yeastbarcode/internal/model"

// LockAncestor marks the founding-generation cluster(s) as the trusted ancestor
// clone barcode(s). Locked ancestors are the reference against which later
// generations are compared; a barcode appearing in a descendant that is NOT in
// the ancestor set and not explainable by inheritance is a contamination signal.
//
// A lineage admits exactly one founding generation: once any generation has
// been locked as the ancestor, locking a different generation is rejected with
// model.ErrAncestorAlreadyLocked. Locking the already-locked generation again is
// idempotent and succeeds without error.
func (s *Service) LockAncestor(lineageID string, generation int) error {
	if err := s.st.EnsureLineageMutable(lineageID); err != nil {
		return err
	}
	clusters, err := s.st.ListClusters(lineageID)
	if err != nil {
		return err
	}
	// reject locking a second, distinct founding generation; re-locking the same
	// generation is idempotent and must succeed.
	for _, c := range clusters {
		if c.IsAncestor && c.Generation != generation {
			return model.NewDomainError("LockAncestor", model.ErrAncestorAlreadyLocked)
		}
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
