package tool

import "testing"

func TestNormalizePlan(t *testing.T) {
	p := upsertPlanInput{Title: "  x  ", Tasks: []planTask{
		{ID: "", Title: "a", Status: "RUNNING"},
		{ID: "t2", Title: "b", Status: "nope"},
	}}
	normalizePlan(&p)
	if p.Title != "x" {
		t.Fatalf("title=%q", p.Title)
	}
	if p.Tasks[0].ID == "" || p.Tasks[0].Status != "running" {
		t.Fatalf("task0=%+v", p.Tasks[0])
	}
	if p.Tasks[1].Status != "pending" {
		t.Fatalf("task1 status=%q", p.Tasks[1].Status)
	}
}
