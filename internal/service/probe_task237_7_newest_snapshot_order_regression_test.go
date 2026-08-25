package service

import (
 "testing"
 snapservice "task237-yeastbarcode/internal/snapshot"
 "task237-yeastbarcode/internal/model"
 "task237-yeastbarcode/internal/store"
)

func makeDrafts(t *testing.T, st *store.Store, id string) (*Services, string, string) { t.Helper();svc:=New(st);if _,e:=svc.CreateLineage(id,id);e!=nil{t.Fatal(e)};if e:=st.SaveCluster(&model.CorrectionCluster{ID:id+"-c",LineageID:id,Generation:0,CanonicalBarcode:"ACGT",ReadCount:1});e!=nil{t.Fatal(e)};a,e:=svc.PublishSnapshot(id,"first");if e!=nil{t.Fatal(e)};b,e:=svc.PublishSnapshot(id,"second");if e!=nil{t.Fatal(e)};return svc,a.ID,b.ID }

func TestTask237Bug07OnlyNewestSnapshotCanBePublishedThroughEveryEntry(t *testing.T) {
 st,err:=store.Open(t.TempDir()+"/publication.db");if err!=nil{t.Fatal(err)};defer st.Close()
 svc,old,newer:=makeDrafts(t,st,"L-service-7");if err:=svc.ConfirmSnapshot(old);err==nil{t.Fatal("service published old draft")};if err:=svc.ConfirmSnapshot(newer);err!=nil{t.Fatal(err)}
 _,old2,new2:=makeDrafts(t,st,"L-domain-7");direct:=snapservice.New(st);if err:=direct.Confirm(old2);err==nil{t.Fatal("domain published old draft")};if err:=direct.Confirm(new2);err!=nil{t.Fatal(err)}
}
