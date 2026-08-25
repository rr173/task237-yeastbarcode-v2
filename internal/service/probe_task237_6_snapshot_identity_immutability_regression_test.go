package service

import (
 "testing"
 "task237-yeastbarcode/internal/model"
 "task237-yeastbarcode/internal/store"
)

func TestTask237Bug06RepeatedPublishKeepsIndependentIdentityAndContent(t *testing.T) {
 st,err:=store.Open(t.TempDir()+"/snapshots.db");if err!=nil{t.Fatal(err)};defer st.Close();svc:=New(st);if _,err:=svc.CreateLineage("L-snapshot-6","snapshot");err!=nil{t.Fatal(err)}
 if err:=st.SaveCluster(&model.CorrectionCluster{ID:"cluster-6",LineageID:"L-snapshot-6",Generation:0,CanonicalBarcode:"ACGT",ReadCount:2});err!=nil{t.Fatal(err)}
 first,err:=svc.PublishSnapshot("L-snapshot-6","first");if err!=nil{t.Fatal(err)};second,err:=svc.PublishSnapshot("L-snapshot-6","second");if err!=nil{t.Fatal(err)};if first.ID==second.ID{t.Fatalf("reused id %q",first.ID)}
 snaps,err:=st.ListSnapshots("L-snapshot-6");if err!=nil{t.Fatal(err)};if len(snaps)!=2{t.Fatalf("snapshots=%d",len(snaps))};old,err:=st.GetSnapshot(first.ID);if err!=nil{t.Fatal(err)};if old.Summary!="first"{t.Fatalf("old summary=%q",old.Summary)}
}
