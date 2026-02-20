package attendance

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/utils"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	ErrCodeExpired        = errors.New("code expired")
	ErrInvalidCode        = errors.New("invalid code")
	ErrCodeForWrongCohort = errors.New("code is for a different cohort")
	ErrAlreadySubmitted   = errors.New("already submitted for this session")
	ErrSessionLocked      = errors.New("attendance for this session is locked")
	ErrStudentNotFound    = errors.New("student not found")
	ErrAllFieldsRequired  = errors.New("code and cohort are required")
	ErrNoActiveCode       = errors.New("no active code for this session")
	ErrRecordNotFound     = errors.New("attendance record not found")
)

type CodeService struct {
	codeRepo    domain.AttendanceCodeRepository
	recordRepo  domain.AttendanceRepository
	userService UserServiceInterface
}

type UserServiceInterface interface {
	GetUserByID(id string) (*domain.User, error)
	GetAllUsers(cohort int, role, email, search, sort string, sortDir, page, limit int) ([]domain.User, int, error)
}

func NewCodeService(codeRepo domain.AttendanceCodeRepository, recordRepo domain.AttendanceRepository, userService UserServiceInterface) *CodeService {
	return &CodeService{
		codeRepo:    codeRepo,
		recordRepo:  recordRepo,
		userService: userService,
	}
}

func (s *CodeService) GenerateCode(cohort int, session domain.AttendanceSession, generatedBy string) (*domain.AttendanceCode, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	code := s.generateRandomCode(string(session))

	now := utils.GetThailandTime()
	expiresAt := now.Add(120 * time.Minute)

	fmt.Printf("Generating code: cohort=%d, session=%s, code=%s, expiresAt=%v\n", cohort, session, code, expiresAt)

	s.codeRepo.DeactivateOldCodes(ctx, cohort, session)

	newCode := &domain.AttendanceCode{
		Code:         code,
		CohortNumber: cohort,
		Session:      session,
		GeneratedAt:  now,
		ExpiresAt:    expiresAt,
		IsActive:     true,
		GeneratedBy:  generatedBy,
	}

	if err := s.codeRepo.InsertCode(ctx, newCode); err != nil {
		return nil, err
	}

	fmt.Printf("Code generated and saved: %+v\n", newCode)
	return newCode, nil
}

func (s *CodeService) generateRandomCode(prefix string) string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	r := rand.New(rand.NewSource(utils.GetThailandTime().UnixNano()))
	code := make([]byte, 4)
	for i := range code {
		code[i] = charset[r.Intn(len(charset))]
	}
	return strings.ToUpper(prefix) + "-" + string(code)
}

func (s *CodeService) GetActiveCode(cohort int, session domain.AttendanceSession) (*domain.AttendanceCode, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.codeRepo.FindActiveCode(ctx, cohort, session)
}

func (s *CodeService) SubmitAttendance(userID primitive.ObjectID, code string, cohort int, ipAddress string) (*domain.AttendanceRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if code == "" || cohort == 0 {
		return nil, ErrAllFieldsRequired
	}

	code = strings.ToUpper(code)
	parts := strings.Split(code, "-")
	if len(parts) != 2 {
		return nil, ErrInvalidCode
	}

	sessionStr := strings.ToLower(parts[0])
	var session domain.AttendanceSession
	if sessionStr == "morning" {
		session = domain.SessionMorning
	} else if sessionStr == "afternoon" {
		session = domain.SessionAfternoon
	} else {
		return nil, ErrInvalidCode
	}

	attendanceCode, err := s.GetActiveCode(cohort, session)
	if err != nil {
		return nil, err
	}

	if attendanceCode == nil {
		return nil, ErrNoActiveCode
	}

	if attendanceCode.Code != code {
		return nil, ErrInvalidCode
	}

	user, err := s.userService.GetUserByID(userID.Hex())
	if err != nil {
		return nil, ErrStudentNotFound
	}

	if user.CohortNumber != cohort {
		return nil, ErrCodeForWrongCohort
	}

	today := utils.GetThailandDate()

	existing, err := s.recordRepo.CountRecords(ctx, domain.AttendanceRecordFilter{
		UserID:     userID,
		Date:       today,
		Session:    session,
		NotDeleted: true,
	})
	if err != nil {
		return nil, err
	}
	if existing > 0 {
		return nil, ErrAlreadySubmitted
	}

	status := s.calculateStatus(session)

	record := &domain.AttendanceRecord{
		UserID:       userID,
		JSDNumber:    user.JSDNumber,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		CohortNumber: user.CohortNumber,
		Date:         today,
		Session:      session,
		Status:       status,
		MarkedBy:     domain.MarkedBySelf,
		SubmittedAt:  time.Now(),
		Locked:       false,
		IPAddress:    ipAddress,
	}

	if err := s.recordRepo.InsertRecord(ctx, record); err != nil {
		return nil, err
	}

	return record, nil
}

func (s *CodeService) calculateStatus(session domain.AttendanceSession) domain.AttendanceStatus {
	var startTime time.Time
	now := utils.GetThailandTime()
	location := now.Location()

	if session == domain.SessionMorning {
		startTime = time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, location)
	} else {
		startTime = time.Date(now.Year(), now.Month(), now.Day(), 13, 0, 0, 0, location)
	}

	elapsed := now.Sub(startTime)

	if elapsed <= 15*time.Minute {
		return domain.StatusPresent
	} else if elapsed <= 90*time.Minute {
		return domain.StatusLate
	} else {
		return domain.StatusAbsent
	}
}
