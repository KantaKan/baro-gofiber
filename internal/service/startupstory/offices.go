package startupstory

import (
	"fmt"

	"gofiber-baro/internal/domain"
)

type OfficeInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Desks int    `json:"desks"`
	Price int    `json:"price"`
	Act   int    `json:"act"`
}

var offices = []OfficeInfo{
	{ID: "garage", Name: "Garage", Desks: 2, Price: 0, Act: 1},
	{ID: "shophouse", Name: "Shophouse", Desks: 4, Price: 1000, Act: 2},
	{ID: "floor", Name: "Office Floor", Desks: 6, Price: 4000, Act: 3},
	{ID: "tower", Name: "Tech Tower", Desks: 8, Price: 12000, Act: 4},
}

func officeIndex(id string) int {
	for i, o := range offices {
		if o.ID == id {
			return i
		}
	}
	return -1
}

func ensureOffice(run *domain.StartupRun) {
	if officeIndex(run.Office) >= 0 {
		return
	}
	run.Office = offices[0].ID
	for _, o := range offices {
		if o.Desks >= len(run.Desks) {
			run.Office = o.ID
			return
		}
	}
	run.Office = offices[len(offices)-1].ID
}

func officeDesks(run *domain.StartupRun) int {
	ensureOffice(run)
	return offices[officeIndex(run.Office)].Desks
}

func MoveOffice(run *domain.StartupRun, id string) error {
	if run.Stage != domain.StartupStageHub {
		return domain.ErrStartupWrongStage
	}
	ensureDesks(run)
	ensureOffice(run)
	if err := noDebt(run); err != nil {
		return err
	}
	to := officeIndex(id)
	if to <= officeIndex(run.Office) {
		return domain.ErrStartupInvalidChoice
	}
	o := offices[to]
	if run.Act < o.Act {
		return domain.ErrStartupDeskLimit
	}
	if run.Money < o.Price {
		return domain.ErrStartupNoFunds
	}
	run.Money -= o.Price
	run.Office = o.ID
	addLog(run, fmt.Sprintf("Moved into the %s. Room for %d desks now.", o.Name, o.Desks))
	return nil
}
