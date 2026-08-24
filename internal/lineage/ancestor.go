package lineage

import (
	"task237-yeastbarcode/internal/model"
)

// LockAncestor marks the founding-generation cluster(s) as the trusted ancestor
// clone barcode(s). Locked ancestors are the reference against which later
// generations are compared; a barcode appearing in a descendant that is NOT in
// the ancestor set and not explainable by inheritance is a contamination signal.
func (s *Service) LockAncestor(lineageID string, generation int) error {
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
	clusters, err := s.st.ListClusters(lineageID)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, c := range clusters {
		if c.IsAncestor {
			out[c.CanonicalBarcode] = true
		}
	}
	return out, nil
}
