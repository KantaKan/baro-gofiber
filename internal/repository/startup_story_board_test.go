package repository

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func stageOf(t *testing.T, p mongo.Pipeline, i int) (string, bson.M) {
	t.Helper()
	if i >= len(p) || len(p[i]) != 1 {
		t.Fatalf("stage %d malformed: %+v", i, p)
	}
	op := p[i][0].Key
	doc, ok := p[i][0].Value.(bson.M)
	if !ok {
		t.Fatalf("stage %d not a document: %+v", i, p[i])
	}
	return op, doc
}

func TestWeeklyPipelineShape(t *testing.T) {
	p := weeklyLeaderboardPipeline(12, "2026-W40", 100)
	if len(p) != 8 {
		t.Fatalf("expected 8 stages, got %d", len(p))
	}
	op, match := stageOf(t, p, 0)
	if op != "$match" || match["cohort"] != 12 || match["mode"] != "ranked" || match["week_key"] != "2026-W40" || match["status"] != "ended" {
		t.Fatalf("bad weekly match: %+v", match)
	}
	if role, ok := match["role"].(bson.M); !ok || role["$ne"] != "admin" {
		t.Fatalf("weekly must exclude admin runs: %+v", match)
	}
	op, group := stageOf(t, p, 2)
	if op != "$group" || group["_id"] != "$owner_id" {
		t.Fatalf("weekly must group one row per owner: %+v", group)
	}
	for _, want := range []string{"score", "run_id", "outcome"} {
		field, ok := group[want].(bson.M)
		if !ok || field["$first"] == nil {
			t.Fatalf("weekly group must keep best %s: %+v", want, group)
		}
	}
	op, lookup := stageOf(t, p, 3)
	if op != "$lookup" || lookup["from"] != "users" {
		t.Fatalf("weekly must join users for names: %+v", lookup)
	}
	op, project := stageOf(t, p, 5)
	if op != "$project" || project["name"] == nil || project["best_run_id"] == nil {
		t.Fatalf("weekly must project name + best run: %+v", project)
	}
	op, sort := stageOf(t, p, 6)
	if op != "$sort" || sort["score"] != -1 {
		t.Fatalf("weekly must sort by score desc: %+v", sort)
	}
	if len(p[7]) != 1 || p[7][0].Key != "$limit" || p[7][0].Value != 100 {
		t.Fatalf("weekly must limit 100: %+v", p[7])
	}
}

func TestFamePipelineShape(t *testing.T) {
	p := fameLeaderboardPipeline(12, 100)
	if len(p) != 7 {
		t.Fatalf("expected 7 stages, got %d", len(p))
	}
	op, match := stageOf(t, p, 0)
	if op != "$match" || match["cohort"] != 12 {
		t.Fatalf("fame must scope to cohort: %+v", match)
	}
	op, adminFilter := stageOf(t, p, 3)
	if op != "$match" {
		t.Fatalf("fame must filter admins: %+v", adminFilter)
	}
	if role, ok := adminFilter["user.role"].(bson.M); !ok || role["$ne"] != "admin" {
		t.Fatalf("fame must exclude admins: %+v", adminFilter)
	}
	op, sort := stageOf(t, p, 5)
	if op != "$sort" || sort["fame"] != -1 {
		t.Fatalf("fame must sort by fame desc: %+v", sort)
	}
}

func TestGenmateFilter(t *testing.T) {
	self := primitive.NewObjectID()
	f := genmateFilter(12, self)
	if f["cohort_number"] != 12 {
		t.Fatalf("must scope to the learner cohort number: %+v", f)
	}
	if role, ok := f["role"].(bson.M); !ok || role["$ne"] != "admin" {
		t.Fatalf("must exclude admins: %+v", f)
	}
	if id, ok := f["_id"].(bson.M); !ok || id["$ne"] != self {
		t.Fatalf("must exclude self: %+v", f)
	}
	if opt, ok := f["startup_story_opt_out"].(bson.M); !ok || opt["$ne"] != true {
		t.Fatalf("must exclude opted-out users: %+v", f)
	}
	if del, ok := f["deleted"].(bson.M); !ok || del["$ne"] != true {
		t.Fatalf("must exclude deleted accounts: %+v", f)
	}
}

func TestGenmateDisplayName(t *testing.T) {
	if genmateDisplayName("Ploy", "JSD12-001") != "Ploy" {
		t.Fatal("first name wins")
	}
	if genmateDisplayName("", "JSD12-001") != "JSD12-001" {
		t.Fatal("gen-id fallback")
	}
}

func TestGenmateProjectionHidesIdentity(t *testing.T) {
	p := genmateProjection()
	if p["first_name"] != 1 || p["jsd_number"] != 1 || p["_id"] != 1 {
		t.Fatalf("projection must carry id + display name: %+v", p)
	}
	if len(p) != 3 {
		t.Fatalf("projection must not expose surname, email or any other field: %+v", p)
	}
}

func TestDeepestPipelineSortsByActThenScore(t *testing.T) {
	p := deepestLeaderboardPipeline(12, "2026-W40", 100)
	op, match := stageOf(t, p, 0)
	if op != "$match" || match["mode"] != "ranked" || match["status"] != "ended" {
		t.Fatalf("bad deepest match: %+v", match)
	}
	if role, ok := match["role"].(bson.M); !ok || role["$ne"] != "admin" {
		t.Fatalf("deepest must exclude admin runs: %+v", match)
	}
	sorts := 0
	for _, stage := range p {
		if stage[0].Key != "$sort" {
			continue
		}
		sorts++
		keys, ok := stage[0].Value.(bson.D)
		if !ok || len(keys) != 2 || keys[0].Key != "depth" || keys[1].Key != "score" {
			t.Fatalf("deepest must sort by depth, then score, in that order: %+v", stage[0].Value)
		}
	}
	if sorts != 2 {
		t.Fatalf("expected a sort before grouping and after the lookup, got %d", sorts)
	}
}
