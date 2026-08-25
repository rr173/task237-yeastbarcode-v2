// Package read validates and ingests raw barcode reads into the store. It is
// the input boundary of the pipeline: illegal barcodes and missing quality are
// rejected before they can pollute the correction stage.
package read

import (
	"errors"
	"time"

	"task237-yeastbarcode/internal/model"
	"task237-yeastbarcode/internal/store"
)

// Service validates and stores barcode reads and generation edges.
type Service struct {
	st *store.Store
}

// New constructs a read Service.
func New(st *store.Store) *Service { return &Service{st: st} }

// IngestRead validates a raw read, computes its content hash for idempotency and
// stores it. It returns the stored read; if an identical read was already
// present it returns model.ErrDuplicate with the existing record attached.
func (s *Service) IngestRead(lineageID string, generation int, rawBarcode string, quality []int) (*model.BarcodeRead, error) {
	if err := model.ValidateBarcode(rawBarcode); err != nil {
		return nil, err
	}
	if err := model.ValidateQuality(rawBarcode, quality); err != nil {
		return nil, err
	}
	hash := model.ReadHash(lineageID, generation, rawBarcode, quality)
	existing, err := s.st.GetReadByHash(hash)
	if err == nil && existing != nil {
		// idempotent: identical content already stored
		return existing, model.ErrDuplicate
	}
	r := &model.BarcodeRead{
		ID:         model.ClusterHash(lineageID, generation, hash)[:16],
		LineageID:  lineageID,
		Generation: generation,
		RawBarcode: rawBarcode,
		Quality:    quality,
		Status:     model.ReadRaw,
		Hash:       hash,
		CreatedAt:  time.Now().UTC(),
	}
	if err := s.st.SaveRead(r); err != nil {
		if errors.Is(err, model.ErrDuplicate) {
			existing, lookupErr := s.st.GetReadByHash(hash)
			if lookupErr != nil {
				return nil, lookupErr
			}
			return existing, model.ErrDuplicate
		}
		return nil, err
	}
	return r, nil
}

// AddGenerationEdge validates and stores a parent->child generation edge. It
// rejects non-increasing generations (cycle guard at the edge level).
func (s *Service) AddGenerationEdge(e model.GenerationEdge) error {
	if err := model.ValidateGenerationEdge(e); err != nil {
		return err
	}
	exists, err := s.st.GenerationEdgeExists(e)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return s.st.SaveGenerationEdge(e)
}
