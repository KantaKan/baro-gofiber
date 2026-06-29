package attendance

import (
	"context"
	"errors"
	"log"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type SubmissionService struct {
	recordRepo  domain.AttendanceRepository
	userService UserServiceInterface
}

func NewSubmissionService(recordRepo domain.AttendanceRepository, userService UserServiceInterface) *SubmissionService {
	return &SubmissionService{
		recordRepo:  recordRepo,
		userService: userService,
	}
}

func (s *SubmissionService) ManualMarkAttendance(userID primitive.ObjectID, date, session string, status domain.AttendanceStatus, markedBy string) (*domain.AttendanceRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user, err := s.userService.GetUserByID(userID.Hex())
	if err != nil {
		log.Printf("[ERROR] ManualMarkAttendance: user not found: %s, err=%v", userID.Hex(), err)
		return nil, ErrStudentNotFound
	}

	now := utils.GetThailandTime()

	update := bson.M{
		"$set": bson.M{
			"user_id":        userID,
			"jsd_number":     user.JSDNumber,
			"first_name":     user.FirstName,
			"last_name":      user.LastName,
			"cohort_number":  user.CohortNumber,
			"date":           date,
			"session":        session,
			"status":         status,
			"marked_by":      domain.MarkedByAdmin,
			"marked_by_user": markedBy,
			"submitted_at":   now,
			"locked":         false,
			"deleted":        false,
			"deleted_at":     nil,
			"deleted_by":     "",
		},
		"$setOnInsert": bson.M{
			"_id": primitive.NewObjectID(),
		},
	}

	filter := domain.AttendanceRecordFilter{
		UserID:     userID,
		Date:       date,
		Session:    domain.AttendanceSession(session),
		NotDeleted: true,
	}

	record, err := s.recordRepo.UpsertRecord(ctx, filter, update)
	if err != nil {
		log.Printf("[ERROR] ManualMarkAttendance upsert failed: %v", err)
		return nil, err
	}

	return record, nil
}

func (s *SubmissionService) BulkMarkAttendance(userIDs []primitive.ObjectID, date string, session domain.AttendanceSession, status domain.AttendanceStatus, markedBy string) ([]domain.AttendanceRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var records []domain.AttendanceRecord
	now := utils.GetThailandTime()

	for _, userID := range userIDs {
		user, err := s.userService.GetUserByID(userID.Hex())
		if err != nil {
			log.Printf("[WARN] BulkMarkAttendance: skipping user %s: not found", userID.Hex())
			continue
		}

		update := bson.M{
			"$set": bson.M{
				"user_id":        userID,
				"jsd_number":     user.JSDNumber,
				"first_name":     user.FirstName,
				"last_name":      user.LastName,
				"cohort_number":  user.CohortNumber,
				"date":           date,
				"session":        session,
				"status":         status,
				"marked_by":      domain.MarkedByAdmin,
				"marked_by_user": markedBy,
				"submitted_at":   now,
				"locked":         false,
				"deleted":        false,
				"deleted_at":     nil,
				"deleted_by":     "",
			},
			"$setOnInsert": bson.M{
				"_id": primitive.NewObjectID(),
			},
		}

		filter := domain.AttendanceRecordFilter{
			UserID:     userID,
			Date:       date,
			Session:    session,
			NotDeleted: true,
		}

		record, err := s.recordRepo.UpsertRecord(ctx, filter, update)
		if err != nil {
			log.Printf("[WARN] BulkMarkAttendance: upsert failed for user %s: %v", userID.Hex(), err)
			continue
		}
		records = append(records, *record)
	}

	return records, nil
}

func (s *SubmissionService) DeleteAttendanceRecord(recordID string, deletedBy string) (*domain.AttendanceRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	oid, err := primitive.ObjectIDFromHex(recordID)
	if err != nil {
		return nil, ErrRecordNotFound
	}

	record, err := s.recordRepo.FindByID(ctx, oid)
	if err != nil {
		return nil, ErrRecordNotFound
	}

	if err := s.recordRepo.DeleteRecord(ctx, oid, deletedBy); err != nil {
		return nil, err
	}

	return record, nil
}

func (s *SubmissionService) LockSession(date, session string, cohort int, locked bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := domain.AttendanceRecordFilter{
		Date:       date,
		Session:    domain.AttendanceSession(session),
		NotDeleted: true,
	}

	if cohort > 0 {
		filter.Cohort = cohort
	}

	update := bson.M{"locked": locked}
	return s.recordRepo.UpdateRecords(ctx, filter, update)
}

func (s *SubmissionService) IsSessionLocked(date, session string, cohort int) (bool, error) {
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

func (s *SubmissionService) GetAttendanceLogs(cohort int, date string, page, limit int) ([]domain.AttendanceRecord, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	filter := domain.AttendanceRecordFilter{
		NotDeleted: true,
	}
	if cohort > 0 {
		filter.Cohort = cohort
	}
	if date != "" {
		filter.Date = date
	}

	total, err := s.recordRepo.CountRecords(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	skip := int64((page - 1) * limit)
	findOpts := options.Find().
		SetSort(bson.D{{Key: "submitted_at", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(limit))

	records, err := s.recordRepo.FindRecords(ctx, filter, findOpts)
	if err != nil {
		return nil, 0, err
	}

	return records, int(total), nil
}

func (s *SubmissionService) GetStudentAttendanceHistory(userID primitive.ObjectID) ([]domain.AttendanceRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	findOpts := options.Find().
		SetSort(bson.D{{Key: "date", Value: -1}, {Key: "session", Value: 1}})

	filter := domain.AttendanceRecordFilter{
		UserID:     userID,
		NotDeleted: true,
	}

	return s.recordRepo.FindRecords(ctx, filter, findOpts)
}

// GetStudentHistorySince returns records for a user starting from (today - days).
func (s *SubmissionService) GetStudentHistorySince(userID primitive.ObjectID, days int) ([]domain.AttendanceRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	startDate := utils.GetThailandTime().AddDate(0, 0, -days).Format("2006-01-02")

	findOpts := options.Find().
		SetSort(bson.D{{Key: "date", Value: -1}, {Key: "session", Value: 1}})

	filter := domain.AttendanceRecordFilter{
		UserID:     userID,
		NotDeleted: true,
	}

	records, err := s.recordRepo.FindRecords(ctx, filter, findOpts)
	if err != nil {
		return nil, err
	}

	filtered := make([]domain.AttendanceRecord, 0, len(records))
	for _, r := range records {
		if r.Date >= startDate {
			filtered = append(filtered, r)
		}
	}
	return filtered, nil
}

func (s *SubmissionService) GetUserAttendanceStatus(userID primitive.ObjectID) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pipeline := []bson.M{
		{"$match": bson.M{
			"user_id": userID,
			"deleted": bson.M{"$ne": true},
		}},
		{"$group": bson.M{
			"_id":            nil,
			"present":        bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "present"}}, 1, 0}}},
			"late":           bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "late"}}, 1, 0}}},
			"absent":         bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "absent"}}, 1, 0}}},
			"late_excused":   bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "late_excused"}}, 1, 0}}},
			"absent_excused": bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "absent_excused"}}, 1, 0}}},
			"total_days":     bson.M{"$sum": 1},
		}},
	}

	stats, err := s.recordRepo.AggregateStats(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"present":        0,
		"late":           0,
		"absent":         0,
		"late_excused":   0,
		"absent_excused": 0,
		"total_days":     0,
		"warning_level":  "normal",
	}

	if len(stats) > 0 {
		result["present"] = stats[0].Present
		result["late"] = stats[0].Late
		result["absent"] = stats[0].Absent
		result["late_excused"] = stats[0].LateExcused
		result["absent_excused"] = stats[0].AbsentExcused
		result["total_days"] = stats[0].Present + stats[0].Late + stats[0].Absent + stats[0].LateExcused + stats[0].AbsentExcused

		if stats[0].Absent >= 7 {
			result["warning_level"] = "red"
		} else if stats[0].Absent >= 4 {
			result["warning_level"] = "yellow"
		}
	}

	return result, nil
}

// ValidateDateFormat returns true if dateStr is YYYY-MM-DD.
func ValidateDateFormat(dateStr string) bool {
	if len(dateStr) != 10 {
		return false
	}
	for i, c := range dateStr {
		if i == 4 || i == 7 {
			if c != '-' {
				return false
			}
		} else if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// isNotFound checks for both the service-level and repo-level not-found errors.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrRecordNotFound) || err.Error() == "attendance record not found"
}
