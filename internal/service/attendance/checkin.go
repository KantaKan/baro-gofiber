package attendance

import (
	"context"
	"errors"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/utils"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	ErrNoClassToday      = errors.New("there is no class today")
	ErrNotOpenYet        = errors.New("check-in is not open yet")
	ErrCheckInClosed     = errors.New("check-in is closed for this session")
	ErrCoveredByLeave    = errors.New("you have approved leave for this session")
	ErrNetworkNotAllowed = errors.New("check-in is not allowed from this network")
)

type CheckInInput struct {
	Now              time.Time
	Rules            CheckinRules
	IsClassDay       bool
	AlreadyCheckedIn bool
	IPAllowed        bool
	// Leave is "" or the excused status an approved leave gives this session.
	Leave domain.AttendanceStatus
}

type CheckInDecision struct {
	Session domain.AttendanceSession
	Status  domain.AttendanceStatus
}

func DecideCheckIn(in CheckInInput) (CheckInDecision, error) {
	if !in.IsClassDay {
		return CheckInDecision{}, ErrNoClassToday
	}
	session := in.Rules.SessionAt(in.Now)
	var status domain.AttendanceStatus
	switch in.Rules.Window(session).PhaseAt(in.Now) {
	case PhaseNotOpen:
		return CheckInDecision{}, ErrNotOpenYet
	case PhaseClosed:
		return CheckInDecision{}, ErrCheckInClosed
	case PhaseOnTime:
		status = domain.StatusPresent
	case PhaseLate:
		status = domain.StatusLate
	}
	if !in.IPAllowed {
		return CheckInDecision{}, ErrNetworkNotAllowed
	}
	if in.AlreadyCheckedIn {
		return CheckInDecision{}, ErrAlreadySubmitted
	}
	switch in.Leave {
	case domain.StatusAbsentExcused:
		return CheckInDecision{}, ErrCoveredByLeave
	case domain.StatusLateExcused:
		if status == domain.StatusLate {
			status = domain.StatusLateExcused
		}
	}
	return CheckInDecision{Session: session, Status: status}, nil
}

// LeaveStatusFor returns the excused status approved leave gives (date, session), or "".
// full_day covers both sessions; half_day covers its session; late covers its session
// (morning when no session was given).
func LeaveStatusFor(leaves []domain.LeaveRequest, date string, s domain.AttendanceSession) domain.AttendanceStatus {
	result := domain.AttendanceStatus("")
	for _, l := range leaves {
		if l.Status != domain.LeaveStatusApproved || l.Date != date {
			continue
		}
		ls := domain.SessionMorning
		if l.Session != nil {
			ls = *l.Session
		}
		switch l.Type {
		case domain.LeaveTypeFullDay:
			return domain.StatusAbsentExcused
		case domain.LeaveTypeHalfDay:
			if ls == s {
				return domain.StatusAbsentExcused
			}
		case domain.LeaveTypeLate:
			if ls == s {
				result = domain.StatusLateExcused
			}
		}
	}
	return result
}

type CohortSettingsGetter interface {
	Get(cohort int) (domain.CohortSettings, error)
}

type HolidayChecker interface {
	IsHoliday(date string) (bool, *domain.Holiday, error)
}

type IPCheck func(allowed []string, ip string) bool

type CheckinService struct {
	records   domain.AttendanceRepository
	leaves    domain.LeaveRequestRepository
	users     UserServiceInterface
	settings  CohortSettingsGetter
	holidays  HolidayChecker
	ipAllowed IPCheck
	now       func() time.Time
}

func NewCheckinService(records domain.AttendanceRepository, leaves domain.LeaveRequestRepository, users UserServiceInterface, settings CohortSettingsGetter, holidays HolidayChecker, ipAllowed IPCheck) *CheckinService {
	return &CheckinService{records: records, leaves: leaves, users: users, settings: settings, holidays: holidays, ipAllowed: ipAllowed, now: utils.GetThailandTime}
}

// IsClassDay: Monday–Friday and not a holiday.
func IsClassDay(now time.Time, holidays HolidayChecker) (bool, error) {
	if wd := now.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return false, nil
	}
	h, _, err := holidays.IsHoliday(utils.FormatDate(now))
	if err != nil {
		return false, err
	}
	return !h, nil
}

func (s *CheckinService) CheckIn(userID primitive.ObjectID, ip, style string) (*domain.AttendanceRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	now := s.now()
	date := utils.FormatDate(now)

	user, err := s.users.GetUserByID(userID.Hex())
	if err != nil {
		return nil, ErrStudentNotFound
	}
	settings, err := s.settings.Get(user.CohortNumber)
	if err != nil {
		return nil, err
	}
	classDay, err := IsClassDay(now, s.holidays)
	if err != nil {
		return nil, err
	}
	session := settings.Rules.SessionAt(now)
	n, err := s.records.CountRecords(ctx, domain.AttendanceRecordFilter{UserID: userID, Date: date, Session: session, NotDeleted: true})
	if err != nil {
		return nil, err
	}
	leaves, err := s.leaves.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	d, err := DecideCheckIn(CheckInInput{
		Now: now, Rules: settings.Rules, IsClassDay: classDay, AlreadyCheckedIn: n > 0,
		IPAllowed: s.ipAllowed(settings.AllowedNetworks, ip), Leave: LeaveStatusFor(leaves, date, session),
	})
	if err != nil {
		return nil, err
	}
	rec := &domain.AttendanceRecord{
		UserID: userID, JSDNumber: user.JSDNumber, FirstName: user.FirstName, LastName: user.LastName,
		CohortNumber: user.CohortNumber, Date: date, Session: d.Session, Status: d.Status,
		MarkedBy: domain.MarkedBySelf, SubmittedAt: time.Now(), IPAddress: ip, CheckinStyle: style,
	}
	if err := s.records.InsertRecord(ctx, rec); err != nil {
		return nil, err
	}
	return rec, nil
}
