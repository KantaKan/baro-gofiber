package attendance

import (
	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// LateRun counts consecutive late days from the newest, skipping holidays and no-class days.
func LateRun(days []DayResult) int {
	run := 0
	for i := len(days) - 1; i >= 0; i-- {
		s := days[i].Status
		if s == domain.StatusHoliday || s == domain.StatusNoClass {
			continue
		}
		if s != domain.StatusLate {
			break
		}
		run++
	}
	return run
}

// ComputeStats derives a learner's totals from their live attendance records.
func ComputeStats(records []domain.AttendanceRecord) domain.LearnerStats {
	days := BuildDays(records)
	s := domain.LearnerStats{
		AttendanceRate: AttendanceRate(days),
		LeaveUsed:      LeaveDaysUsed(days),
		Streak:         OnTimeStreak(days),
		LateRun:        LateRun(days),
		Last10:         last(days, 10),
	}
	for _, r := range records {
		if r.MarkedBy == domain.MarkedBySelf && !r.SubmittedAt.IsZero() && (s.LastSeen == nil || r.SubmittedAt.After(*s.LastSeen)) {
			t := r.SubmittedAt
			s.LastSeen = &t
		}
	}
	return s
}

// StatsByUser computes stats for every learner that appears in records.
func StatsByUser(records []domain.AttendanceRecord) map[primitive.ObjectID]domain.LearnerStats {
	byUser := map[primitive.ObjectID][]domain.AttendanceRecord{}
	cohort := map[primitive.ObjectID]int{}
	for _, r := range records {
		byUser[r.UserID] = append(byUser[r.UserID], r)
		cohort[r.UserID] = r.CohortNumber
	}
	out := make(map[primitive.ObjectID]domain.LearnerStats, len(byUser))
	for id, rs := range byUser {
		s := ComputeStats(rs)
		s.UserID, s.CohortNumber = id, cohort[id]
		out[id] = s
	}
	return out
}
