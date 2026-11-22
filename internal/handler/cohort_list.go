package handler

import (
	"sort"

	"gofiber-baro/internal/service/staff"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

// CohortInfo is one entry in the admin cohort picker.
type CohortInfo struct {
	Cohort    int    `json:"cohort"`
	Learners  int    `json:"learners"`
	StartDate string `json:"start_date"`
}

type cohortListSource interface {
	LearnerCounts() (map[int]int, error)
	StartDates() (map[int]string, error)
}

// MergeCohorts lists every cohort that has learners or settings, newest first.
func MergeCohorts(learners map[int]int, starts map[int]string) []CohortInfo {
	seen := map[int]bool{}
	out := []CohortInfo{}
	add := func(n int) {
		if n <= 0 || seen[n] {
			return
		}
		seen[n] = true
		out = append(out, CohortInfo{Cohort: n, Learners: learners[n], StartDate: starts[n]})
	}
	for n := range learners {
		add(n)
	}
	for n := range starts {
		add(n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Cohort > out[j].Cohort })
	return out
}

// CohortListHandler serves the cohort picker.
type CohortListHandler struct{ src cohortListSource }

func NewCohortListHandler(s cohortListSource) *CohortListHandler { return &CohortListHandler{src: s} }

// List: cohorts for the switcher (non-owners only see their own).
// @Summary  Cohorts for the picker  @Tags admin  @Security BearerAuth  @Router /admin/cohorts [get]
func (h *CohortListHandler) List(c *fiber.Ctx) error {
	counts, err := h.src.LearnerCounts()
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Couldn't load cohorts")
	}
	starts, err := h.src.StartDates()
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Couldn't load cohorts")
	}
	all := MergeCohorts(counts, starts)
	s, _ := StaffFrom(c)
	if s.Role == staff.Owner {
		return utils.SendResponse(c, fiber.StatusOK, "ok", all)
	}
	mine := []CohortInfo{}
	for _, ci := range all {
		if staff.InScope(s.Role, s.Cohorts, ci.Cohort) {
			mine = append(mine, ci)
		}
	}
	return utils.SendResponse(c, fiber.StatusOK, "ok", mine)
}
