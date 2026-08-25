package discriminate

import (
	"fmt"
	"time"

	"task237-yeastbarcode/internal/correction"
	"task237-yeastbarcode/internal/model"
	"task237-yeastbarcode/internal/store"
)

// Service analyses generations for foreign-barcode contamination.
type Service struct {
	st  *store.Store
	cfg Config
}

// New constructs a discrimination Service.
func New(st *store.Store, cfg Config) *Service { return &Service{st: st, cfg: cfg} }

// AnalyzeGeneration compares the corrected clusters of a generation against the
// inherited expectation and produces contamination candidates. Foreign
// barcodes (not in ancestor set and not within InheritTolerance of an inherited
// barcode) whose frequency exceeds MinFrequency become candidates.
func (s *Service) AnalyzeGeneration(lineageID string, generation int) ([]*model.ContaminationCandidate, error) {
	if err := s.st.EnsureLineageMutable(lineageID); err != nil {
		return nil, err
	}
	clusters, err := s.st.ListClusters(lineageID)
	if err != nil {
		return nil, err
	}
	// total usable reads across this generation for frequency
	genClusters := []*model.CorrectionCluster{}
	total := 0
	for _, c := range clusters {
		if c.Generation == generation {
			genClusters = append(genClusters, c)
			total += c.ReadCount
		}
	}
	if total == 0 {
		return nil, fmt.Errorf("discriminate: no clusters at generation %d", generation)
	}
	// inherited expectation = ancestor barcodes + parent generation barcodes
	ancestorSet, err := s.ancestorSet(lineageID)
	if err != nil {
		return nil, err
	}
	inherited := map[string]bool{}
	for b := range ancestorSet {
		inherited[b] = true
	}
	parentBarcodes, err := s.parentBarcodes(lineageID, generation)
	if err != nil {
		return nil, err
	}
	for b := range parentBarcodes {
		inherited[b] = true
	}

	cands := []*model.ContaminationCandidate{}
	for _, c := range genClusters {
		freq := float64(c.ReadCount) / float64(total)
		if freq < s.cfg.MinFrequency {
			continue
		}
		if isInherited(c.CanonicalBarcode, inherited, s.cfg.InheritTolerance) {
			continue
		}
		cand := &model.ContaminationCandidate{
			ID:            model.ClusterHash(lineageID, generation, "cand:"+c.CanonicalBarcode),
			LineageID:     lineageID,
			Generation:    generation,
			Barcode:       c.CanonicalBarcode,
			EvidenceScore: evidenceScore(freq, c.ReadCount),
			Frequency:     freq,
			Source:        "foreign_barcode",
			Status:        model.CandGenerated,
		}
		if existing, lookupErr := s.st.GetCandidate(cand.ID); lookupErr == nil {
			if existing.Status == model.CandConfirmed || existing.Status == model.CandRejected {
				cands = append(cands, existing)
				continue
			}
		} else if lookupErr != model.ErrNotFound {
			return nil, lookupErr
		}
		if err := s.st.SaveCandidate(cand); err != nil {
			return nil, err
		}
		cands = append(cands, cand)
	}
	return cands, nil
}

// evidenceScore combines frequency and absolute read support into a single
// comparable score in [0,1]. Higher frequency and higher absolute count both
// raise confidence that the barcode is real, not a residual error.
func evidenceScore(freq float64, count int) float64 {
	countFactor := 1.0 - 1.0/float64(count+1)
	score := freq*0.7 + countFactor*0.3
	if score > 1 {
		score = 1
	}
	return score
}

func isInherited(bc string, inherited map[string]bool, tol int) bool {
	if inherited[bc] {
		return true
	}
	for exp := range inherited {
		if tol > 0 && correction.HammingDistance(bc, exp) >= 0 && correction.HammingDistance(bc, exp) <= tol {
			return true
		}
	}
	return false
}

func (s *Service) ancestorSet(lineageID string) (map[string]bool, error) {
	// reuse lineage service via store directly
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

func (s *Service) parentBarcodes(lineageID string, generation int) (map[string]bool, error) {
	edges, err := s.st.ListGenerationEdges(lineageID)
	if err != nil {
		return nil, err
	}
	parents := map[int]bool{}
	for _, e := range edges {
		if e.ChildGeneration == generation {
			parents[e.ParentGeneration] = true
		}
	}
	clusters, err := s.st.ListClusters(lineageID)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, c := range clusters {
		if parents[c.Generation] {
			out[c.CanonicalBarcode] = true
		}
	}
	return out, nil
}

// DecideCandidate transitions a candidate to confirmed/rejected with a note.
// The transition is applied as a single compare-and-set UPDATE conditioned on
// the candidate's current status, so two researchers deciding the same
// candidate concurrently cannot both succeed: the first writer flips the
// status and the second's conditional UPDATE matches zero rows and returns
// model.ErrStateConflict. This gives the verdict one-time submission
// semantics — the candidate is updated at most once.
func (s *Service) DecideCandidate(id string, confirm bool, note string) error {
	target, err := s.st.GetCandidate(id)
	if err != nil {
		return err
	}
	if err := s.st.EnsureLineageMutable(target.LineageID); err != nil {
		return err
	}
	to := model.CandRejected
	if confirm {
		to = model.CandConfirmed
	}
	if !model.ValidCandidateTransition(target.Status, to) {
		return fmt.Errorf("discriminate: invalid candidate transition %s->%s", target.Status, to)
	}
	decided := time.Now().UTC().Format(time.RFC3339)
	changed, err := s.st.TransitionCandidate(id, target.Status, to, note, decided)
	if err != nil {
		return err
	}
	if !changed {
		// Another request already moved this candidate out of the expected
		// status between our read and our conditional UPDATE.
		return model.ErrStateConflict
	}
	return nil
}
