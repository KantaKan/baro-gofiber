package handler

import (
	"context"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
)

var actionNames = map[string]string{
	"POST /admin/attendance/mark":                                 "Marked attendance",
	"POST /admin/attendance/:id/restore":                          "Undid an attendance change",
	"POST /admin/attendance/manual":                               "Marked attendance",
	"POST /admin/attendance/bulk":                                 "Marked attendance for several learners",
	"DELETE /admin/attendance/:id":                                "Removed an attendance record",
	"PATCH /admin/leave-requests/:id":                             "Reviewed a leave request",
	"POST /admin/leave-requests":                                  "Added leave for a learner",
	"PUT /admin/cohorts/:cohort/settings":                         "Changed check-in rules",
	"PUT /admin/cohorts/:cohort/groups":                           "Moved learners between groups",
	"POST /admin/cohorts":                                         "Created a cohort",
	"POST /admin/learners/:id/notes":                              "Added a TA note",
	"DELETE /admin/notes/:noteId":                                 "Deleted a TA note",
	"POST /admin/learners/:id/support":                            "Added a support step",
	"PATCH /admin/support/:itemId":                                "Updated a support step",
	"DELETE /admin/support/:itemId":                               "Deleted a support step",
	"POST /admin/learners/:id/support/ack":                        "Acknowledged a support reason",
	"POST /admin/notifications":                                   "Sent an announcement",
	"PUT /admin/notifications/:id":                                "Changed an announcement",
	"DELETE /admin/notifications/:id":                             "Deleted an announcement",
	"POST /admin/holidays":                                        "Added a holiday",
	"DELETE /admin/holidays/:id":                                  "Removed a holiday",
	"POST /admin/cup/awards":                                      "Awarded Cup points",
	"DELETE /admin/cup/awards/:id":                                "Undid Cup points",
	"POST /admin/users/:id/gift-boxes":                            "Gave a gift box",
	"POST /admin/cohorts/:cohortNumber/gift-boxes":                "Gave gift boxes to a cohort",
	"POST /admin/users/bulk-register":                             "Imported learners",
	"PUT /admin/users/:userId/reflections/:reflectionId/feedback": "Replied to a reflection",
	"POST /admin/users/:id/badges":                                "Awarded a badge",
	"DELETE /admin/users/:id":                                     "Deleted a user",
}

// DescribeAction turns an audited request into a sentence a person can read.
func DescribeAction(method, route string) string {
	if s, ok := actionNames[method+" "+route]; ok {
		return s
	}
	return method + " " + route
}

type auditPager interface {
	FindPage(ctx context.Context, filter bson.M, page, limit int) ([]domain.AuditLog, int64, error)
}

type ActivityHandler struct {
	audit auditPager
	users careUserLookup
}

func NewActivityHandler(a auditPager, u careUserLookup) *ActivityHandler {
	return &ActivityHandler{audit: a, users: u}
}

type activityItem struct {
	ID        string            `json:"id"`
	ActorName string            `json:"actor_name"`
	ActorRole string            `json:"actor_role"`
	Action    string            `json:"action"`
	Detail    string            `json:"detail"`
	Status    int               `json:"status"`
	At        time.Time         `json:"at"`
	Targets   []string          `json:"targets,omitempty"`
	Facts     map[string]string `json:"facts,omitempty"`
}

// List is the readable admin activity log (writes only, newest first).
// @Summary  Activity log  @Tags admin  @Security BearerAuth  @Router /admin/activity [get]
func (h *ActivityHandler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	if page < 1 {
		page = 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	logs, total, err := h.audit.FindPage(ctx, bson.M{"method": bson.M{"$ne": "GET"}, "actor_role": bson.M{"$ne": "learner"}}, page, 40)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading activity")
	}
	names := map[string]string{}
	name := func(id string) string {
		if _, ok := names[id]; !ok {
			names[id] = ""
			if u, e := h.users.GetUserByID(id); e == nil && u != nil {
				names[id] = u.FirstName
			}
		}
		return names[id]
	}
	items := make([]activityItem, 0, len(logs))
	for _, l := range logs {
		actor := name(l.ActorID.Hex())
		if actor == "" {
			actor = "Someone"
		}
		var targets []string
		for _, t := range l.Targets {
			// route ids can be records, not people; only names that resolve are shown
			if n := name(t); n != "" {
				targets = append(targets, n)
			}
		}
		items = append(items, activityItem{ID: l.ID.Hex(), ActorName: actor, ActorRole: l.ActorRole, Action: DescribeAction(l.Method, l.Route), Detail: l.Path, Status: l.Status, At: l.CreatedAt, Targets: targets, Facts: l.Detail})
	}
	return utils.SendResponse(c, fiber.StatusOK, "Activity", fiber.Map{"items": items, "total": total, "page": page})
}
