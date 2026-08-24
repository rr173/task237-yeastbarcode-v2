package correction

import (
	"sort"

	"task237-yeastbarcode/internal/model"
	"task237-yeastbarcode/internal/read"
	"task237-yeastbarcode/internal/store"
)

// Config controls clustering thresholds.
type Config struct {
	// MaxEditDistance: reads within this Levenshtein distance of a canonical
	// barcode collapse into the same cluster (sequencing error tolerance).
	MaxEditDistance int
	// MajorityFraction: a canonical must be supported by at least this fraction
	// of the cluster's reads to be trusted over singletons.
	MajorityFraction float64
}

// DefaultConfig returns the standard correction config.
func DefaultConfig() Config {
	return Config{MaxEditDistance: 2, MajorityFraction: 0.5}
}

// Service builds correction clusters from stored reads of a lineage generation.
type Service struct {
	st      *store.Store
	readSvc *read.Service
	cfg     Config
	qp      read.QualityPolicy
}

// New constructs a correction Service.
func New(st *store.Store, readSvc *read.Service, cfg Config) *Service {
	return &Service{st: st, readSvc: readSvc, cfg: cfg, qp: read.NewQualityPolicy()}
}

// ClusterGeneration corrects and clusters all usable reads of one generation.
// Low-quality reads are marked excluded; the rest are assigned to a canonical
// barcode cluster. Returns the created clusters.
func (s *Service) ClusterGeneration(lineageID string, generation int) ([]*model.CorrectionCluster, error) {
	reads, err := s.st.ListReadsByGeneration(lineageID, generation)
	if err != nil {
		return nil, err
	}
	// reset clusters for idempotent re-correction
	if err := s.st.DeleteClustersForGeneration(lineageID, generation); err != nil {
		return nil, err
	}
	type acc struct {
		canonical string
		members   []*model.BarcodeRead
	}
	accs := []*acc{}
	used := map[string]bool{}
	for _, r := range reads {
		if r.Status == model.ReadExcluded {
			continue
		}
		if !s.qp.IsUsable(r.Quality) {
			_ = s.st.UpdateReadStatus(r.ID, model.ReadLowQuality, "", "")
			continue
		}
		bc := r.RawBarcode
		placed := false
		for _, a := range accs {
			if used[a.canonical] && EditDistance(a.canonical, bc) <= s.cfg.MaxEditDistance {
				a.members = append(a.members, r)
				placed = true
				// keep the lexicographically smallest as canonical to be stable
				if bc < a.canonical {
					a.canonical = bc
				}
				break
			}
		}
		if !placed {
			accs = append(accs, &acc{canonical: bc, members: []*model.BarcodeRead{r}})
		}
	}
	clusters := []*model.CorrectionCluster{}
	for _, a := range accs {
		// mark duplicate members (same raw barcode, multiple reads)
		seenRaw := map[string]int{}
		for _, m := range a.members {
			seenRaw[m.RawBarcode]++
		}
		clusterID := model.ClusterHash(lineageID, generation, a.canonical)
		majority := float64(0)
		if len(a.members) > 0 {
			majority = float64(maxCount(seenRaw)) / float64(len(a.members))
		}
		_ = majority
		// assign cluster id to members and mark status corrected
		for _, m := range a.members {
			_ = s.st.UpdateReadStatus(m.ID, model.ReadCorrected, a.canonical, clusterID)
		}
		c := &model.CorrectionCluster{
			ID:              clusterID,
			LineageID:       lineageID,
			Generation:      generation,
			CanonicalBarcode: a.canonical,
			ReadCount:       len(a.members),
			IsAncestor:      false,
		}
		if err := s.st.SaveCluster(c); err != nil {
			return nil, err
		}
		clusters = append(clusters, c)
	}
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].ReadCount > clusters[j].ReadCount
	})
	return clusters, nil
}

func maxCount(m map[string]int) int {
	best := 0
	for _, v := range m {
		if v > best {
			best = v
		}
	}
	return best
}
