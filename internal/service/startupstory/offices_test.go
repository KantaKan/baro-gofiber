package startupstory

import (
	"errors"
	"testing"

	"gofiber-baro/internal/domain"
)

func TestOfficeMoves(t *testing.T) {
	run := newHubRun(t, 91)
	if run.Office != "garage" || officeDesks(run) != 2 {
		t.Fatalf("runs start in the 2-desk garage, got %q", run.Office)
	}
	if err := MoveOffice(run, "shophouse"); !errors.Is(err, domain.ErrStartupDeskLimit) {
		t.Fatalf("the shophouse opens in act 2, got %v", err)
	}
	run.Act = 2
	run.Money = offices[1].Price - 1
	if err := MoveOffice(run, "shophouse"); !errors.Is(err, domain.ErrStartupNoFunds) {
		t.Fatalf("expected no funds, got %v", err)
	}
	run.Money = 100000
	run.Desks = []int{3, 1}
	if err := MoveOffice(run, "shophouse"); err != nil {
		t.Fatal(err)
	}
	if officeDesks(run) != 4 || run.Desks[0] != 3 || run.Money != 100000-offices[1].Price {
		t.Fatalf("moving keeps desks and costs the price: %+v money %d", run.Desks, run.Money)
	}
	if err := MoveOffice(run, "garage"); !errors.Is(err, domain.ErrStartupInvalidChoice) {
		t.Fatalf("no moving back down, got %v", err)
	}
	if err := MoveOffice(run, "tower"); !errors.Is(err, domain.ErrStartupDeskLimit) {
		t.Fatalf("the tower opens in act 4, got %v", err)
	}
}

func TestOldRunsGetAnOfficeThatFitsTheirDesks(t *testing.T) {
	for desks, want := range map[int]string{2: "garage", 3: "shophouse", 4: "shophouse", 6: "floor", 8: "tower"} {
		run := &domain.StartupRun{Desks: make([]int, desks)}
		ensureOffice(run)
		if run.Office != want {
			t.Errorf("%d desks: got %q, want %q", desks, run.Office, want)
		}
	}
}
