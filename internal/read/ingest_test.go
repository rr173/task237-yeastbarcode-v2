package read

import (
	"errors"
	"testing"

	"task237-yeastbarcode/internal/model"
	"task237-yeastbarcode/internal/store"
)

func TestIngestReadIsIdempotentAndValidatesInput(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/reads.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	svc := New(st)
	quality := []int{35, 35, 35, 35}
	first, err := svc.IngestRead("L-read", 0, "ACGT", quality)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.IngestRead("L-read", 0, "ACGT", quality)
	if !errors.Is(err, model.ErrDuplicate) {
		t.Fatalf("duplicate error=%v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("duplicate returned id=%q want %q", second.ID, first.ID)
	}
	if _, err := svc.IngestRead("L-read", 0, "ACGX", quality); !errors.Is(err, model.ErrBarcodeIllegal) {
		t.Fatalf("illegal barcode error=%v", err)
	}
}
