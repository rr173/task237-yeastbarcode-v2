package snapshot

import (
 "sync"
 "testing"
 "task237-yeastbarcode/internal/model"
 "task237-yeastbarcode/internal/store"
)

func TestTask237Bug09ConcurrentSnapshotConfirmationUsesCAS(t *testing.T) {
 st,err:=store.Open(t.TempDir()+"/snapshot.db");if err!=nil{t.Fatal(err)};defer st.Close();if err:=st.SaveLineage(&model.CultureLineage{ID:"L-snapshot-9",Name:"snapshot",Status:model.LineageFiled});err!=nil{t.Fatal(err)};snap,err:=New(st).Publish("L-snapshot-9","race");if err!=nil{t.Fatal(err)};svc:=New(st);start:=make(chan struct{});errs:=make(chan error,20);var wg sync.WaitGroup;for i:=0;i<20;i++{wg.Add(1);go func(){defer wg.Done();<-start;errs<-svc.Confirm(snap.ID)}()};close(start);wg.Wait();close(errs);success,fail:=0,0;for e:=range errs{if e==nil{success++}else{fail++}};if success!=1||fail!=19{t.Fatalf("success=%d fail=%d",success,fail)}
}
