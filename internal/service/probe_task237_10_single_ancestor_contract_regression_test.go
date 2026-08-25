package service

import (
 "testing"
 "task237-yeastbarcode/internal/model"
 "task237-yeastbarcode/internal/store"
)

func TestTask237Bug10SingleAncestorContractIsEnforced(t *testing.T) {
 st,err:=store.Open(t.TempDir()+"/ancestor.db");if err!=nil{t.Fatal(err)};defer st.Close();svc:=New(st);if _,err:=svc.CreateLineage("L-ancestor-10","ancestor");err!=nil{t.Fatal(err)};if err:=st.SaveCluster(&model.CorrectionCluster{ID:"founding-10",LineageID:"L-ancestor-10",Generation:0,CanonicalBarcode:"ACGT",ReadCount:3});err!=nil{t.Fatal(err)};if err:=st.SaveCluster(&model.CorrectionCluster{ID:"child-10",LineageID:"L-ancestor-10",Generation:1,CanonicalBarcode:"TTTT",ReadCount:3});err!=nil{t.Fatal(err)}
 if err:=svc.LockAncestor("L-ancestor-10",0);err!=nil{t.Fatal(err)};if err:=svc.LockAncestor("L-ancestor-10",1);err==nil{t.Fatal("second ancestor accepted")};a,err:=svc.AncestorBarcodes("L-ancestor-10");if err!=nil{t.Fatal(err)};if !a["ACGT"]||a["TTTT"]{t.Fatalf("ancestors=%#v",a)}
}
