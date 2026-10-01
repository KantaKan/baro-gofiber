package startupstory

import (
	"context"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func hasFounder(pool []domain.StartupDev, title string) bool {
	for _, f := range pool {
		if f.Title == title {
			return true
		}
	}
	return false
}

func TestSecretFounderUnlocksAfterThreeOSSShips(t *testing.T) {
	studio := &domain.StartupStudio{OSSShips: OSSUnlockShips - 1}
	if hasFounder(founderPoolFor(studio), ossFounder.Title) || ossFounderUnlocked(studio) {
		t.Fatal("the secret founder must stay hidden before the unlock")
	}
	studio.OSSShips = OSSUnlockShips
	if !hasFounder(founderPoolFor(studio), ossFounder.Title) || !ossFounderUnlocked(studio) {
		t.Fatal("shipping 3 OSS products should reveal the secret founder")
	}
	if !IsOSSProduct("Dev Tool/CLI") || !IsOSSProduct("Open-Source Library") || IsOSSProduct("Web App") {
		t.Fatal("only Dev Tool/CLI and Open-Source Library count toward the unlock")
	}
}

func ossRun(t *testing.T, oss bool) *domain.StartupRun {
	t.Helper()
	pool := []domain.StartupDev{founders[0], founders[1], founders[2]}
	if oss {
		pool = []domain.StartupDev{ossFounder, ossFounder, ossFounder}
	}
	run := newRunWithPool(primitive.NewObjectID(), 12, "learner", domain.StartupModeFree, 5, t0, pool, nil)
	run.Market = domain.StartupMarket{}
	if err := PickFounder(run, 0); err != nil {
		t.Fatal(err)
	}
	run.Staff[0].Frontend, run.Staff[0].Backend, run.Staff[0].Design, run.Staff[0].Debug = 6, 6, 6, 6
	if err := StartProject(run, "Web App", "Education", nil, t0); err != nil {
		t.Fatal(err)
	}
	if err := Ship(run, run.Project.EndsAt); err != nil {
		t.Fatal(err)
	}
	return run
}

func TestOSSRunUsesStarsSponsorsAndOSSReviewers(t *testing.T) {
	normal, oss := ossRun(t, false), ossRun(t, true)
	if normal.OSS || !oss.OSS {
		t.Fatalf("only the Open Source Maintainer starts an OSS run: %v %v", normal.OSS, oss.OSS)
	}
	names := map[string]bool{}
	for _, r := range oss.LastResult.Reviews {
		names[r.Reviewer] = true
	}
	for _, want := range []string{"Maintainers", "Contributors", "Hacker News", "Big Tech"} {
		if !names[want] {
			t.Fatalf("OSS runs are reviewed by the OSS crowd, missing %s: %+v", want, oss.LastResult.Reviews)
		}
	}
	total := oss.LastResult.Total
	if oss.LastResult.FansDelta != int(float64(total*total*FansPerPoint)*OSSFansMult+0.5) {
		t.Fatalf("OSS fans (stars) should be boosted: %+v", oss.LastResult)
	}
}

func TestOSSReviewersKeepTheirRoles(t *testing.T) {
	for _, rv := range ossReviewers {
		found := false
		for _, std := range reviewers {
			if rv.role() == std.Name {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s must stand in for a standard reviewer so roles, perks and items still apply", rv.Name)
		}
	}
}

func TestShippingThreeDevToolsUnlocksTheSecretFounder(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{}
	svc := &Service{store: store, now: func() time.Time { return t0 }}
	player := Player{ID: primitive.NewObjectID().Hex(), Cohort: 12, Role: "learner"}
	if _, err := svc.StartRun(ctx, player, domain.StartupModeFree); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PickFounder(ctx, player, 0); err != nil {
		t.Fatal(err)
	}
	for shipped := 0; shipped < OSSUnlockShips; {
		switch store.run.Stage {
		case domain.StartupStageHub:
			store.run.Staff[0].Burnout = 0
			store.run.Staff[0].Frontend, store.run.Staff[0].Backend, store.run.Staff[0].Design, store.run.Staff[0].Debug = 30, 30, 30, 30
			if _, err := svc.StartProject(ctx, player, "Dev Tool/CLI", "Productivity", nil); err != nil {
				t.Fatal(err)
			}
			store.run.Project.EndsAt = t0.Add(-time.Second)
			if _, err := svc.Ship(ctx, player); err != nil {
				t.Fatal(err)
			}
			shipped++
		case domain.StartupStageItem:
			_, _ = svc.PickItem(ctx, player, 0)
		case domain.StartupStagePerk:
			_, _ = svc.PickPerk(ctx, player, 0)
		default:
			t.Fatalf("unexpected stage %s", store.run.Stage)
		}
	}
	overview, err := svc.Overview(ctx, player)
	if err != nil {
		t.Fatal(err)
	}
	if !overview.OSSUnlocked || store.studio.OSSShips != OSSUnlockShips {
		t.Fatalf("3 Dev Tool ships should unlock the secret founder: %+v", store.studio)
	}
}
