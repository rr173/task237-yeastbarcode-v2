package service

import (
 "errors"
 "testing"
 "task237-yeastbarcode/internal/model"
 "task237-yeastbarcode/internal/store"
)

func TestTask237Bug04SealedLineageRejectsAllMutatingPipelineOperations(t *testing.T) {
 st,err:=store.Open(t.TempDir()+"/sealed.db");if err!=nil{t.Fatal(err)};defer st.Close();svc:=New(st)
 if _,err:=svc.CreateLineage("L-sealed-4","sealed");err!=nil{t.Fatal(err)};q:=[]int{35,35,35,35}
 if _,err:=svc.IngestRead("L-sealed-4",0,"ACGT",q);err!=nil{t.Fatal(err)};if _,err:=svc.CorrectGeneration("L-sealed-4",0);err!=nil{t.Fatal(err)};if err:=svc.LockAncestor("L-sealed-4",0);err!=nil{t.Fatal(err)};if _,err:=svc.TransitionLineage("L-sealed-4",model.LineageSealed);err!=nil{t.Fatal(err)}
 checks:=[]func()error{func()error{_,e:=svc.IngestRead("L-sealed-4",1,"TTTT",q);return e},func()error{return svc.AddGenerationEdge(model.GenerationEdge{LineageID:"L-sealed-4",ParentGeneration:0,ChildGeneration:1})},func()error{_,e:=svc.CorrectGeneration("L-sealed-4",0);return e},func()error{return svc.LockAncestor("L-sealed-4",0)},func()error{_,e:=svc.AnalyzeGeneration("L-sealed-4",0);return e},func()error{_,e:=svc.PublishSnapshot("L-sealed-4","sealed");return e}}
 for i,call:=range checks{if !errors.Is(call(),model.ErrSealedMutate){t.Errorf("check %d was not rejected",i)}}
}
