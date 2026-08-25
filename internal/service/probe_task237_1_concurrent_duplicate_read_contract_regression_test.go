package service

import (
 "errors"
 "sync"
 "testing"
 "task237-yeastbarcode/internal/model"
 "task237-yeastbarcode/internal/store"
)

func TestTask237Bug01ConcurrentDuplicateReadContract(t *testing.T) {
 st, err := store.Open(t.TempDir()+"/reads.db"); if err != nil { t.Fatal(err) }; defer st.Close()
 svc := New(st); if _, err := svc.CreateLineage("L-dup", "dup"); err != nil { t.Fatal(err) }
 start := make(chan struct{}); var wg sync.WaitGroup; var mu sync.Mutex
 successes, duplicates, unexpected := 0, 0, []error{}
 for i:=0;i<20;i++ { wg.Add(1); go func(){ defer wg.Done(); <-start; _, e:=svc.IngestRead("L-dup",0,"ACGT",[]int{35,35,35,35}); mu.Lock(); defer mu.Unlock(); if e==nil { successes++ } else if errors.Is(e,model.ErrDuplicate) { duplicates++ } else { unexpected=append(unexpected,e) } }() }
 close(start); wg.Wait()
 if successes!=1 || duplicates!=19 || len(unexpected)!=0 { t.Fatalf("successes=%d duplicates=%d unexpected=%v",successes,duplicates,unexpected) }
 rows, err := st.ListReadsByLineage("L-dup"); if err != nil { t.Fatal(err) }; if len(rows)!=1 { t.Fatalf("stored reads=%d",len(rows)) }
}
