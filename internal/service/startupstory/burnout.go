package startupstory

import (
	"time"

	"gofiber-baro/internal/domain"
)

func afterShipBurnout(run *domain.StartupRun, team []domain.StartupDev, now time.Time) bool {
	return false
}
