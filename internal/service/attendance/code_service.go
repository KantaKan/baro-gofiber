package attendance

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
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
	ErrTooManyAttempts     = errors.New("too many failed attempts, try again later")
)

// codeAttemptKey is the in-memory throttle entry for per-user code brute-force protection.
type codeAttemptKey struct {
	userID  primitive.ObjectID
	session string
}

type codeAttempt struct {
	count    int
	firstTry time.Time
}

// In-memory rate limiter for code submissions. Tolerates burst of 3 wrong codes per
// session, then blocks for 5 minutes. Resets automatically. No external dep needed.
var (
	attemptMu     sync.Mutex
	codeAttempts  = map[codeAttemptKey]codeAttempt{}
	cleanupDone   = make(chan struct{})
	attemptBurst  = 3
	attemptWindow = 5 * time.Minute
)

func init() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				func() {
					attemptMu.Lock()
					defer attemptMu.Unlock()
					now := utils.GetThailandTime()
					for k, a := range codeAttempts {
						if now.Sub(a.firstTry) > attemptWindow {
							delete(codeAttempts, k)
						}
					}
				}()
			case <-cleanupDone:
				return
			}
		}
	}()
}

func checkCodeRateLimit(userID primitive.ObjectID, session string) error {
	attemptMu.Lock()
	defer attemptMu.Unlock()

	key := codeAttemptKey{userID: userID, session: session}
	now := utils.GetThailandTime()

	a, ok := codeAttempts[key]
	if ok && a.count >= attemptBurst && now.Sub(a.firstTry) < attemptWindow {
		remaining := attemptWindow - now.Sub(a.firstTry)
		return fmt.Errorf("%w (%.0f seconds remaining)", ErrTooManyAttempts, remaining.Seconds())
	}

	if !ok {
		codeAttempts[key] = codeAttempt{count: 0, firstTry: now}
	}
	return nil
}

func recordFailedAttempt(userID primitive.ObjectID, session string) {
	attemptMu.Lock()
	defer attemptMu.Unlock()

	key := codeAttemptKey{userID: userID, session: session}
	a, ok := codeAttempts[key]
	if !ok {
		return
	}
	a.count++
	codeAttempts[key] = a
}

func clearAttempts(userID primitive.ObjectID, session string) {
	attemptMu.Lock()
	defer attemptMu.Unlock()

	key := codeAttemptKey{userID: userID, session: session}
	delete(codeAttempts, key)
}

type UserServiceInterface interface {
	GetUserByID(id string) (*domain.User, error)
	GetAllUsers(cohort int, role, email, search, sort string, sortDir, page, limit int, excludeAttendanceStatus ...string) ([]domain.User, int, error)
}

type CodeService struct {
	codeRepo    domain.AttendanceCodeRepository
	recordRepo  domain.AttendanceRepository
	userService UserServiceInterface
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

	return newCode, nil
}

func (s *CodeService) generateRandomCode(prefix string) string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	code := make([]byte, 6)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			code[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		} else {
			code[i] = charset[n.Int64()]
		}
	}
	return strings.ToUpper(prefix) + "-" + string(code)
}

func (s *CodeService) GetActiveCode(cohort int, session domain.AttendanceSession) (*domain.AttendanceCode, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.codeRepo.FindActiveCode(ctx, cohort, session)
}

func (s *CodeService) SubmitAttendance(userID primitive.ObjectID, code string, cohort int, ipAddress string) (*domain.AttendanceRecord, error) {
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

	// Rate-limit: reject burst of failed attempts per session.
	if err := checkCodeRateLimit(userID, string(session)); err != nil {
		return nil, err
	}

	attendanceCode, err := s.GetActiveCode(cohort, session)
	if err != nil {
		return nil, err
	}

	if attendanceCode == nil {
		return nil, ErrNoActiveCode
	}

	if attendanceCode.Code != code {
		recordFailedAttempt(userID, string(session))
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

	// Enforce session lock.
	locked, err := s.IsSessionLocked(today, string(session), cohort)
	if err != nil {
		return nil, err
	}
	if locked {
		return nil, ErrSessionLocked
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check for existing live record (double-submit guard).
	existing, err := s.recordRepo.FindRecord(ctx, domain.AttendanceRecordFilter{
		UserID:     userID,
		Date:       today,
		Session:    session,
		NotDeleted: true,
	})
	if err != nil && !isNotFound(err) {
		return nil, err
	}
	if existing != nil {
		return nil, ErrAlreadySubmitted
	}

	// Re-check lock right before insert to close TOCTOU window.
	stillLocked, err := s.IsSessionLocked(today, string(session), cohort)
	if err != nil {
		return nil, err
	}
	if stillLocked {
		return nil, ErrSessionLocked
	}

	status := s.calculateStatus(session)
	now := utils.GetThailandTime()

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
		SubmittedAt:  now,
		Locked:       false,
		IPAddress:    ipAddress,
	}

	if err := s.recordRepo.InsertRecord(ctx, record); err != nil {
		// Partial-unique index means this should not happen for live records,
		// but handle gracefully just in case.
		if strings.Contains(err.Error(), "E11000") || strings.Contains(err.Error(), "duplicate key") {
			return nil, ErrAlreadySubmitted
		}
		return nil, err
	}

	clearAttempts(userID, string(session))
	return record, nil
}

// IsSessionLocked exposes the lock check for use from SubmitAttendance.
func (s *CodeService) IsSessionLocked(date, session string, cohort int) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := domain.AttendanceRecordFilter{
		Date:       date,
		Session:    domain.AttendanceSession(session),
		Cohort:     cohort,
		NotDeleted: true,
	}

	records, err := s.recordRepo.FindRecords(ctx, filter, nil)
	if err != nil {
		return false, err
	}

	for _, r := range records {
		if r.Locked {
			return true, nil
		}
	}
	return false, nil
}

func (s *CodeService) calculateStatus(session domain.AttendanceSession) domain.AttendanceStatus {
	now := utils.GetThailandTime()
	location := now.Location()

	var startTime time.Time
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
