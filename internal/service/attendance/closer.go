package attendance

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/utils"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// PlanSessionClose decides the record each learner without a check-in gets when a
// session closes: their approved-leave status if any, otherwise absent.
func PlanSessionClose(learners []primitive.ObjectID, checkedIn map[primitive.ObjectID]bool, leave map[primitive.ObjectID]domain.AttendanceStatus) map[primitive.ObjectID]domain.AttendanceStatus {
	out := map[primitive.ObjectID]domain.AttendanceStatus{}
	for _, id := range learners {
		if checkedIn[id] {
			continue
		}
		if s, ok := leave[id]; ok && s != "" {
			out[id] = s
		} else {
			out[id] = domain.StatusAbsent
		}
	}
	return out
}

type closerRecords interface {
	FindRecords(ctx interface{}, filter domain.AttendanceRecordFilter, opts interface{}) ([]domain.AttendanceRecord, error)
	UpsertRecord(ctx interface{}, record *domain.AttendanceRecord) (bool, error)
	ActiveCohorts(ctx interface{}, since string) ([]int, error)
}

type closerLeaves interface {
	FindAll(ctx interface{}, filter domain.LeaveRequestFilter) ([]domain.LeaveRequest, error)
}

// SessionCloser marks everyone who didn't check in once a session's window closes.
type SessionCloser struct {
	records  closerRecords
	leaves   closerLeaves
	users    UserServiceInterface
	settings CohortSettingsGetter
	holidays HolidayChecker

	mu     sync.Mutex
	closed map[string]bool // "cohort|date|session"
}

func NewSessionCloser(records closerRecords, leaves closerLeaves, users UserServiceInterface, settings CohortSettingsGetter, holidays HolidayChecker) *SessionCloser {
	return &SessionCloser{records: records, leaves: leaves, users: users, settings: settings, holidays: holidays, closed: map[string]bool{}}
}

func (s *SessionCloser) CloseIfDue(now time.Time) error {
	classDay, err := IsClassDay(now, s.holidays)
	if err != nil || !classDay {
		return err
	}
	learners, _, err := s.users.GetAllUsers(0, "learner", "", "", "email", 1, 0, 0, "dropout,dismissed")
	if err != nil {
		return err
	}
	// Only cohorts with attendance in the last 7 days are running; alumni are left alone.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	active, err := s.records.ActiveCohorts(ctx, utils.FormatDate(now.AddDate(0, 0, -7)))
	cancel()
	if err != nil {
		return err
	}
	isActive := map[int]bool{}
	for _, c := range active {
		isActive[c] = true
	}
	byCohort := map[int][]domain.User{}
	for _, u := range learners {
		if isActive[u.CohortNumber] {
			byCohort[u.CohortNumber] = append(byCohort[u.CohortNumber], u)
		}
	}
	date := utils.FormatDate(now)
	var firstErr error
	for cohort, users := range byCohort {
		settings, err := s.settings.Get(cohort)
		if err != nil {
			log.Printf("session closer: cohort %d settings: %v", cohort, err)
			firstErr = errors.Join(firstErr, err)
			continue
		}
		for _, session := range []domain.AttendanceSession{domain.SessionMorning, domain.SessionAfternoon} {
			key := fmt.Sprintf("%d|%s|%s", cohort, date, session)
			s.mu.Lock()
			done := s.closed[key]
			s.mu.Unlock()
			if done || settings.Rules.Window(session).PhaseAt(now) != PhaseClosed {
				continue
			}
			if err := s.closeSession(cohort, date, session, users); err != nil {
				// Keep going so one bad row can't stop other sessions or cohorts.
				log.Printf("session closer: cohort %d %s %s: %v", cohort, date, session, err)
				firstErr = errors.Join(firstErr, err)
				continue
			}
			s.mu.Lock()
			s.closed[key] = true
			s.mu.Unlock()
		}
	}
	return firstErr
}

func (s *SessionCloser) closeSession(cohort int, date string, session domain.AttendanceSession, users []domain.User) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	recs, err := s.records.FindRecords(ctx, domain.AttendanceRecordFilter{Cohort: cohort, Date: date, Session: session, NotDeleted: true}, nil)
	if err != nil {
		return err
	}
	checkedIn := map[primitive.ObjectID]bool{}
	for _, r := range recs {
		checkedIn[r.UserID] = true
	}
	leaves, err := s.leaves.FindAll(ctx, domain.LeaveRequestFilter{Cohort: cohort, Status: domain.LeaveStatusApproved})
	if err != nil {
		return err
	}
	byUser := map[primitive.ObjectID][]domain.LeaveRequest{}
	for _, l := range leaves {
		byUser[l.UserID] = append(byUser[l.UserID], l)
	}
	ids := make([]primitive.ObjectID, 0, len(users))
	info := map[primitive.ObjectID]domain.User{}
	leaveStatus := map[primitive.ObjectID]domain.AttendanceStatus{}
	for _, u := range users {
		ids = append(ids, u.ID)
		info[u.ID] = u
		if st := LeaveStatusFor(byUser[u.ID], date, session); st != "" {
			leaveStatus[u.ID] = st
		}
	}
	for id, status := range PlanSessionClose(ids, checkedIn, leaveStatus) {
		u := info[id]
		rec := &domain.AttendanceRecord{
			UserID: id, JSDNumber: u.JSDNumber, FirstName: u.FirstName, LastName: u.LastName,
			CohortNumber: cohort, Date: date, Session: session, Status: status,
			MarkedBy: domain.MarkedByAdmin, MarkedByUser: "system", SubmittedAt: time.Now(),
		}
		if _, err := s.records.UpsertRecord(ctx, rec); err != nil {
			return err
		}
	}
	return nil
}
