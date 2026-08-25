package service

import (
 "testing"
 "task237-yeastbarcode/internal/model"
 "task237-yeastbarcode/internal/store"
)

func TestTask237Bug05DAGShortcutAndReplayAreAccepted(t *testing.T) {
 st,err:=store.Open(t.TempDir()+"/graph.db");if err!=nil{t.Fatal(err)};defer st.Close();svc:=New(st);if _,err:=svc.CreateLineage("L-graph-5","graph");err!=nil{t.Fatal(err)}
 edges:=[]model.GenerationEdge{{LineageID:"L-graph-5",ParentGeneration:0,ChildGeneration:1},{LineageID:"L-graph-5",ParentGeneration:1,ChildGeneration:2},{LineageID:"L-graph-5",ParentGeneration:0,ChildGeneration:2},{LineageID:"L-graph-5",ParentGeneration:0,ChildGeneration:2}}
 for _,e:=range edges{if err:=svc.AddGenerationEdge(e);err!=nil{t.Fatalf("edge %+v: %v",e,err)}};got,err:=st.ListGenerationEdges("L-graph-5");if err!=nil{t.Fatal(err)};if len(got)!=3{t.Fatalf("edges=%d want 3",len(got))}
}
