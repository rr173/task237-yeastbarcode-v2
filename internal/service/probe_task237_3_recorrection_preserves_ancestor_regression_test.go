package service

import (
 "testing"
 "task237-yeastbarcode/internal/model"
 "task237-yeastbarcode/internal/store"
)

func TestTask237Bug03RecorrectionPreservesAncestorLock(t *testing.T) {
 st,err:=store.Open(t.TempDir()+"/recorrection.db");if err!=nil{t.Fatal(err)};defer st.Close();svc:=New(st)
 if _,err:=svc.CreateLineage("L-recorrect-3","recorrection");err!=nil{t.Fatal(err)}
 for i:=0;i<3;i++{if _,err:=svc.IngestRead("L-recorrect-3",0,"ACGT",[]int{35,35,35,35});err!=nil&&err!=model.ErrDuplicate{t.Fatal(err)}}
 if _,err:=svc.CorrectGeneration("L-recorrect-3",0);err!=nil{t.Fatal(err)};if err:=svc.LockAncestor("L-recorrect-3",0);err!=nil{t.Fatal(err)}
 if _,err:=svc.CorrectGeneration("L-recorrect-3",0);err!=nil{t.Fatal(err)};a,err:=svc.AncestorBarcodes("L-recorrect-3");if err!=nil{t.Fatal(err)};if !a["ACGT"]{t.Fatalf("ancestors=%#v",a)}
}
