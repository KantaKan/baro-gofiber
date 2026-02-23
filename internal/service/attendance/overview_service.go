package attendance

import (
	"context"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/utils"
)

type OverviewService struct {
	recordRepo  domain.AttendanceRepository
	codeRepo    domain.AttendanceCodeRepository
	userService UserServiceInterface
}

func NewOverviewService(recordRepo domain.AttendanceRepository, codeRepo domain.AttendanceCodeRepository, userService UserServiceInterface) *OverviewService {
	return &OverviewService{
		recordRepo:  recordRepo,
		codeRepo:    codeRepo,
		userService: userService,
	}
}

func (s *OverviewService) GetTodayAttendanceOverview(cohort int, session domain.AttendanceSession) (*domain.TodayAttendanceOverview, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	today := utils.GetThailandDate()

	var activeCode *domain.AttendanceCode
	if session != "" {
		activeCode, _ = s.codeRepo.FindActiveCode(ctx, cohort, session)
	}

	filter := domain.AttendanceRecordFilter{
		Cohort:     cohort,
		Date:       today,
		NotDeleted: true,
	}

	if session != "" {
		filter.Session = session
	}

	records, err := s.recordRepo.FindRecords(ctx, filter, nil)
	if err != nil {
		return nil, err
	}

	type sessionInfo struct {
		Status string
		ID     string
	}
	submittedMap := make(map[string]map[string]sessionInfo)
	for _, r := range records {
		key := r.UserID.Hex()
		if submittedMap[key] == nil {
			submittedMap[key] = make(map[string]sessionInfo)
		}
		submittedMap[key][string(r.Session)] = sessionInfo{
			Status: string(r.Status),
			ID:     r.ID.Hex(),
		}
	}

	users, _, err := s.userService.GetAllUsers(cohort, "", "", "", "first_name", 1, 1, 500)
	if err != nil {
		return nil, err
	}

	students := make([]domain.StudentAttendanceRow, 0, len(users))
	for _, user := range users {
		row := domain.StudentAttendanceRow{
			UserID:    user.ID,
			JSDNumber: user.JSDNumber,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Morning:   "-",
			Afternoon: "-",
		}

		if sessionData, ok := submittedMap[user.ID.Hex()]; ok {
			if m, ok := sessionData["morning"]; ok {
				row.Morning = m.Status
				row.MorningRecordID = m.ID
			}
			if a, ok := sessionData["afternoon"]; ok {
				row.Afternoon = a.Status
				row.AfternoonRecordID = a.ID
			}
		}

		students = append(students, row)
	}

	overview := &domain.TodayAttendanceOverview{
		Session:        session,
		SubmittedCount: len(records),
		Students:       students,
	}

	if activeCode != nil {
		overview.Code = activeCode.Code
		overview.ExpiresAt = activeCode.ExpiresAt
	}

	return overview, nil
}

func (s *OverviewService) GetAttendanceOverviewByDate(cohort int, session domain.AttendanceSession, date string) (*domain.TodayAttendanceOverview, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	targetDate := date
	if targetDate == "" {
		targetDate = utils.GetThailandDate()
	}

	var activeCode *domain.AttendanceCode
	if session != "" && targetDate == utils.GetThailandDate() {
		activeCode, _ = s.codeRepo.FindActiveCode(ctx, cohort, session)
	}

	filter := domain.AttendanceRecordFilter{
		Cohort:     cohort,
		Date:       targetDate,
		NotDeleted: true,
	}

	if session != "" {
		filter.Session = session
	}

	records, err := s.recordRepo.FindRecords(ctx, filter, nil)
	if err != nil {
		return nil, err
	}

	type sessionInfo struct {
		Status string
		ID     string
	}
	submittedMap := make(map[string]map[string]sessionInfo)
	for _, r := range records {
		key := r.UserID.Hex()
		if submittedMap[key] == nil {
			submittedMap[key] = make(map[string]sessionInfo)
		}
		submittedMap[key][string(r.Session)] = sessionInfo{
			Status: string(r.Status),
			ID:     r.ID.Hex(),
		}
	}

	users, _, err := s.userService.GetAllUsers(cohort, "", "", "", "first_name", 1, 1, 500)
	if err != nil {
		return nil, err
	}

	students := make([]domain.StudentAttendanceRow, 0, len(users))
	for _, user := range users {
		row := domain.StudentAttendanceRow{
			UserID:    user.ID,
			JSDNumber: user.JSDNumber,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Morning:   "-",
			Afternoon: "-",
		}

		if sessionData, ok := submittedMap[user.ID.Hex()]; ok {
			if m, ok := sessionData["morning"]; ok {
				row.Morning = m.Status
				row.MorningRecordID = m.ID
			}
			if a, ok := sessionData["afternoon"]; ok {
				row.Afternoon = a.Status
				row.AfternoonRecordID = a.ID
			}
		}

		students = append(students, row)
	}

	overview := &domain.TodayAttendanceOverview{
		Session:        session,
		SubmittedCount: len(records),
		Students:       students,
	}

	if activeCode != nil {
		overview.Code = activeCode.Code
		overview.ExpiresAt = activeCode.ExpiresAt
	}

	return overview, nil
}
