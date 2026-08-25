package service

import (
 "testing"
 "task237-yeastbarcode/internal/model"
 "task237-yeastbarcode/internal/store"
)

func TestTask237Bug02ReanalysisPreservesDecisionAcrossLayers(t *testing.T) {
 st, err := store.Open(t.TempDir()+"/analysis.db"); if err != nil { t.Fatal(err) }; defer st.Close()
 svc:=New(st); if _,err:=svc.CreateLineage("L-reanalysis-2","reanalysis"); err!=nil { t.Fatal(err) }
 if err:=st.SaveCluster(&model.CorrectionCluster{ID:"ancestor",LineageID:"L-reanalysis-2",Generation:0,CanonicalBarcode:"ACGT",ReadCount:10,IsAncestor:true});err!=nil{t.Fatal(err)}
 if err:=st.SaveCluster(&model.CorrectionCluster{ID:"foreign",LineageID:"L-reanalysis-2",Generation:1,CanonicalBarcode:"TTTT",ReadCount:5});err!=nil{t.Fatal(err)}
 got,err:=svc.AnalyzeGeneration("L-reanalysis-2",1);if err!=nil||len(got)!=1{t.Fatalf("first analysis: %v %d",err,len(got))}
 if err:=svc.DecideCandidate(got[0].ID,true,"reviewed");err!=nil{t.Fatal(err)}
 refreshed,err:=svc.AnalyzeGeneration("L-reanalysis-2",1);if err!=nil||len(refreshed)!=1{t.Fatalf("reanalysis: %v %d",err,len(refreshed))}
 if refreshed[0].Status!=model.CandConfirmed||refreshed[0].VerdictNote!="reviewed"{t.Fatalf("returned decision=%s note=%q",refreshed[0].Status,refreshed[0].VerdictNote)}
 stored,err:=svc.GetCandidate(got[0].ID);if err!=nil{t.Fatal(err)};if stored.Status!=model.CandConfirmed||stored.VerdictNote!="reviewed"{t.Fatalf("stored decision=%s note=%q",stored.Status,stored.VerdictNote)}
}
