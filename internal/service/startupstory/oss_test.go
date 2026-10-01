package startupstory

import (
	"testing"

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
