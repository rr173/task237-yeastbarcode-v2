package discriminate

import (
 "sync"
 "testing"
 "task237-yeastbarcode/internal/model"
 "task237-yeastbarcode/internal/store"
)

func TestTask237Bug08ConcurrentCandidateDecisionsUseCAS(t *testing.T) {
 st,err:=store.Open(t.TempDir()+"/candidate.db");if err!=nil{t.Fatal(err)};defer st.Close();if err:=st.SaveLineage(&model.CultureLineage{ID:"L-candidate-8",Name:"candidate",Status:model.LineageFiled});err!=nil{t.Fatal(err)};c:=&model.ContaminationCandidate{ID:"candidate-8",LineageID:"L-candidate-8",Generation:1,Barcode:"TTTT",EvidenceScore:.8,Frequency:.2,Source:"test",Status:model.CandGenerated};if err:=st.SaveCandidate(c);err!=nil{t.Fatal(err)}
 svc:=New(st,DefaultConfig());start:=make(chan struct{});errs:=make(chan error,2);var wg sync.WaitGroup;for _,confirm:=range []bool{true,false}{wg.Add(1);go func(v bool){defer wg.Done();<-start;errs<-svc.DecideCandidate(c.ID,v,"race")}(confirm)};close(start);wg.Wait();close(errs);success,fail:=0,0;for e:=range errs{if e==nil{success++}else{fail++}};if success!=1||fail!=1{t.Fatalf("success=%d fail=%d",success,fail)}
}
