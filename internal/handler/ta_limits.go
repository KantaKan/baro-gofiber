package handler

import (
	"context"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/attendance"
	"gofiber-baro/internal/service/staff"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TALeaveLimit: a TA can approve leave only up to this many of the 8 days.
const TALeaveLimit = 6

func taMayApprove(role, status string, projected int) bool {
	return role != staff.TA || status != string(domain.LeaveStatusApproved) || projected <= TALeaveLimit
}

// taMayAnnounce: a TA may only send to named learners in their own cohorts and groups.
func taMayAnnounce(s staff.Ctx, recipients []*domain.User) bool {
	if s.Role != staff.TA {
		return true
	}
	if len(recipients) == 0 {
		return false
	}
	for _, u := range recipients {
		if u == nil || !staff.InScope(s.Role, s.Cohorts, u.CohortNumber) || !contains(s.Groups, u.GenmateGroup) {
			return false
		}
	}
	return true
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x && x != "" {
			return true
		}
	}
	return false
}

// LeaveProjection loads a learner's attendance and projects leave for a date.
type LeaveProjection struct{ Records adminViewRecords }

func (p LeaveProjection) Project(userID primitive.ObjectID, date string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	recs, err := p.Records.FindRecords(ctx, domain.AttendanceRecordFilter{UserID: userID, NotDeleted: true}, nil)
	if err != nil {
		return 0, err
	}
	return attendance.ProjectLeave(attendance.BuildDays(recs), date), nil
}

func staffRole(c *fiber.Ctx) string {
	s, _ := StaffFrom(c)
	return s.Role
}
