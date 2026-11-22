package wellbeing

import (
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func user(name, group string, refls ...domain.Reflection) domain.User {
	return domain.User{ID: primitive.NewObjectID(), FirstName: name, ZoomName: "01_" + name, GenmateGroup: group, Role: "learner", Reflections: refls}
}

var nights10 = []string{"2026-08-27", "2026-08-28", "2026-08-31", "2026-09-01", "2026-09-02", "2026-09-03", "2026-09-04", "2026-09-07", "2026-09-08", "2026-09-09"}

func TestBuildBoard(t *testing.T) {
	a := user("A", "F", refl("2026-09-08", P, ""), refl("2026-09-09", P, "I want to quit"), refl("2026-09-02", C, ""))
	b := user("B", "F", refl("2026-09-09", C, "good day"))
	c := user("C", "C") // nothing written
	d := user("D", "C", refl("2026-09-09", E, ""))
	d.AttendanceStatus = "dropout"
	bd := BuildBoard([]domain.User{a, b, c, d}, nights10, "night", DefaultWords)
	if bd.Date != "2026-09-09" || bd.Sent != 2 || bd.Total != 3 || len(bd.Missing) != 1 || bd.Missing[0].FirstName != "C" {
		t.Fatalf("sent/missing wrong: %+v", bd)
	}
	if bd.Zones["panic"] != 1 || bd.Zones["comfort"] != 1 || bd.ZonesPrev["comfort"] != 1 {
		t.Errorf("zones %v prev %v (prev = same weekday last week 2026-09-02)", bd.Zones, bd.ZonesPrev)
	}
	if bd.Map[0].Learner.FirstName != "A" || len(bd.Map[0].Nights) != 10 || bd.Map[0].Nights[9] != "panic" {
		t.Errorf("map worst first: %+v", bd.Map[0])
	}
	if len(bd.Worth) != 2 || bd.Worth[0].Learner.FirstName != "A" || bd.Worth[0].Quote == "" || bd.Worth[1].Patterns[0].Key != "silent" {
		t.Errorf("worth = %+v", bd.Worth)
	}
	if len(bd.Entries) != 2 || bd.Entries[0].Zone != "panic" || len(bd.Entries[0].Words) == 0 {
		t.Errorf("entries panic first with words: %+v", bd.Entries)
	}
	if len(bd.ByGroup) != 2 || bd.ByGroup[0].Group != "C" || bd.ByGroup[0].Sent != 0 || bd.ByGroup[1].Sent != 2 {
		t.Errorf("by group = %+v", bd.ByGroup)
	}
}

func TestBuildSupportList(t *testing.T) {
	a := user("A", "F", refl("2026-09-08", P, ""), refl("2026-09-09", P, ""))
	b := user("B", "F", refl("2026-09-08", C, ""), refl("2026-09-09", C, ""))
	c := user("C", "C", refl("2026-09-08", C, ""), refl("2026-09-09", C, ""))
	items := []domain.SupportItem{{UserID: c.ID, Text: "1:1"}, {UserID: c.ID, Text: "done", Done: true}}
	acks := []domain.SupportAck{{UserID: a.ID, ReasonKey: "pattern:panic_run", Until: time.Now().Add(time.Hour)}}
	list := BuildSupportList([]domain.User{a, b, c}, nil, items, acks, nights10[8:], DefaultWords)
	if len(list) != 1 || list[0].Learner.FirstName != "C" || list[0].OpenItems != 1 || list[0].DoneItems != 1 {
		t.Fatalf("support list = %+v (A acked, B fine, C has an open step)", list)
	}
}
