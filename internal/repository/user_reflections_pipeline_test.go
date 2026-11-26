package repository

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func TestReflectionsViewKeepsTextForOneNight(t *testing.T) {
	v := reflectionsView(20, "2026-09-08")
	m := v["$map"].(bson.M)
	in := m["input"].(bson.M)["$slice"].(bson.A)
	if in[1] != -20 {
		t.Errorf("slice = %v, want last 20", in[1])
	}
	cond := m["in"].(bson.M)["$cond"].(bson.A)
	eq := cond[0].(bson.M)["$eq"].(bson.A)
	if eq[1] != "2026-09-08" || cond[1] != "$$r" {
		t.Errorf("text night match = %v → %v", eq, cond[1])
	}
	slim := cond[2].(bson.M)
	if _, ok := slim["reflection"].(bson.M)["barometer"]; !ok || len(slim) != 4 {
		t.Errorf("other nights keep %v, want _id/day/date/barometer only", slim)
	}
	day := eq[0].(bson.M)["$cond"].(bson.A)
	if fmt := day[2].(bson.M)["$dateToString"].(bson.M)["format"]; fmt != "%Y-%m-%d" {
		t.Errorf("reflections without day must match on their date, got %v", day[2])
	}
}

func TestReflectionsViewWithoutTextNight(t *testing.T) {
	cond := reflectionsView(5, "")["$map"].(bson.M)["in"].(bson.M)["$cond"].(bson.A)
	if cond[0].(bson.M)["$eq"].(bson.A)[1] == "" {
		t.Error("empty textDay would keep text for undated reflections")
	}
}
