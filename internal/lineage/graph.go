// Package lineage maintains the generation inheritance graph of a culture: it
// connects parent and child generations through stored edges, detects illegal
// cycles and resolves, for each generation, the set of barcodes that are
// expected by inheritance (the "expected" barcode set).
package lineage

import (
	"fmt"

	"task237-yeastbarcode/internal/model"
	"task237-yeastbarcode/internal/store"
)

// Service resolves generation inheritance.
type Service struct {
	st *store.Store
}

// New constructs a lineage Service.
func New(st *store.Store) *Service { return &Service{st: st} }

// DetectCycle returns ErrGenerationCycle if adding edge would create a cycle in
// the lineage's generation graph (transitive reachability check).
//
// A legal cross-generation shortcut edge (e.g. gen 0 -> gen 2 alongside an
// existing 0 -> 1 -> 2 chain) introduces a parallel path but never a cycle, so
// it must be accepted. The cycle condition is narrowly "the child can already
// reach the parent through existing edges"; only then does parent->child close
// a loop. Duplicate edges (child already directly reachable from parent) do
// not satisfy that condition and are therefore not flagged here — the store
// layer handles their idempotent upsert.
func (s *Service) DetectCycle(lineageID string, edge model.GenerationEdge) error {
	edges, err := s.st.ListGenerationEdges(lineageID)
	if err != nil {
		return err
	}
	// build adjacency parent->children
	adj := map[int][]int{}
	for _, e := range edges {
		adj[e.ParentGeneration] = append(adj[e.ParentGeneration], e.ChildGeneration)
	}
	if reachable(adj, edge.ChildGeneration, edge.ParentGeneration) {
		return model.ErrGenerationCycle
	}
	return nil
}

func reachable(adj map[int][]int, from, target int) bool {
	if from == target {
		// A self-referential reachability (child == parent) means the edge
		// closes back onto itself; ValidateGenerationEdge rejects child == parent
		// outright, so a true hit here signals a genuine cycle.
		return true
	}
	seen := map[int]bool{}
	stack := []int{from}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[n] {
			continue
		}
		seen[n] = true
		for _, c := range adj[n] {
			if c == target {
				return true
			}
			if !seen[c] {
				stack = append(stack, c)
			}
		}
	}
	return false
}

// ExpectedBarcodes returns the canonical barcodes expected at a generation by
// inheritance from its direct parents. A generation with no parents (the
// founding generation) has no inherited expectation.
func (s *Service) ExpectedBarcodes(lineageID string, generation int) (map[string]bool, error) {
	edges, err := s.st.ListGenerationEdges(lineageID)
	if err != nil {
		return nil, err
	}
	parents := []int{}
	for _, e := range edges {
		if e.ChildGeneration == generation {
			parents = append(parents, e.ParentGeneration)
		}
	}
	expected := map[string]bool{}
	for _, p := range parents {
		clusters, err := s.st.ListClusters(lineageID)
		if err != nil {
			return nil, err
		}
		for _, c := range clusters {
			if c.Generation == p {
				expected[c.CanonicalBarcode] = true
			}
		}
	}
	return expected, nil
}

// ValidateEdge detects a cycle before delegating to the read service to persist.
func (s *Service) ValidateEdge(edge model.GenerationEdge) error {
	if err := model.ValidateGenerationEdge(edge); err != nil {
		return err
	}
	if err := s.DetectCycle(edge.LineageID, edge); err != nil {
		return fmt.Errorf("lineage: %w", err)
	}
	return nil
}
