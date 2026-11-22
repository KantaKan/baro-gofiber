package handler

import (
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/cup"
	"gofiber-baro/internal/service/report"
	"gofiber-baro/pkg/utils"
)

// ReportSource loads the live data the weekly report is built from, reusing the admin views' queries.
type ReportSource struct {
	WB       *WellbeingViewsHandler
	Holidays interface {
		GetHolidayDatesInRange(startDate, endDate string) (map[string]bool, error)
	}
	Cup interface {
		Standings(cohort int, week string) ([]cup.GroupScore, []domain.CupAward, error)
	}
}

func (s ReportSource) Load(cohort int, week string) (report.Input, error) {
	users, _, err := s.WB.users.GetAllUsers(cohort, "learner", "", "", "jsd_number", 1, 1, 1000)
	if err != nil {
		return report.Input{}, err
	}
	recs, err := s.WB.cohortRecords(cohort)
	if err != nil {
		return report.Input{}, err
	}
	monday, err := time.Parse("2006-01-02", week)
	if err != nil {
		return report.Input{}, err
	}
	hol, err := s.Holidays.GetHolidayDatesInRange(monday.AddDate(0, 0, -49).Format("2006-01-02"), monday.AddDate(0, 0, 4).Format("2006-01-02"))
	if err != nil {
		hol = map[string]bool{} // a holiday lookup failure shouldn't block the report
	}
	scores, _, err := s.Cup.Standings(cohort, week)
	if err != nil {
		scores = nil
	}
	return report.Input{
		Week: week, Today: utils.FormatDate(s.WB.now()), Users: users, Records: recs, Holidays: hol,
		Support: s.WB.SupportRows(users, recs), Cup: scores,
	}, nil
}
