package attendance

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
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

	log.Printf("[DEBUG] ManualMarkAttendance: userID=%s, date=%s, session=%s, status=%s", userID.Hex(), date, session, status)

	user, err := s.userService.GetUserByID(userID.Hex())
	if err != nil {
		log.Printf("[ERROR] ManualMarkAttendance: user not found: %s, err=%v", userID.Hex(), err)
		return nil, ErrStudentNotFound
	}
	log.Printf("[DEBUG] ManualMarkAttendance: user found: %s %s", user.FirstName, user.LastName)

	filter := domain.AttendanceRecordFilter{
		UserID:  userID,
		Date:    date,
		Session: domain.AttendanceSession(session),
	}

	existing, err := s.recordRepo.FindRecord(ctx, filter)
	// Check for record not found - handle both service and repo error instances
	isNotFound := errors.Is(err, ErrRecordNotFound) || (err != nil && err.Error() == "attendance record not found")
	if err != nil && !isNotFound {
		log.Printf("[ERROR] ManualMarkAttendance: FindRecord error: %v", err)
		return nil, err
	}

	if existing != nil && existing.ID != primitive.NilObjectID {
		log.Printf("[DEBUG] ManualMarkAttendance: updating existing record %s", existing.ID.Hex())
		update := bson.M{
			"status":         status,
			"marked_by":      domain.MarkedByAdmin,
			"marked_by_user": markedBy,
			"submitted_at":   time.Now(),
			"deleted":        false,
			"deleted_at":     nil,
			"deleted_by":     "",
		}
		if err := s.recordRepo.UpdateRecord(ctx, existing.ID, update); err != nil {
			log.Printf("[ERROR] ManualMarkAttendance: UpdateRecord error: %v", err)
			return nil, err
		}
		existing.Status = status
		existing.MarkedBy = domain.MarkedByAdmin
		existing.MarkedByUser = markedBy
		existing.Deleted = false
		return existing, nil
	}

	record := &domain.AttendanceRecord{
		UserID:       userID,
		JSDNumber:    user.JSDNumber,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		CohortNumber: user.CohortNumber,
		Date:         date,
		Session:      domain.AttendanceSession(session),
		Status:       status,
		MarkedBy:     domain.MarkedByAdmin,
		MarkedByUser: markedBy,
		SubmittedAt:  time.Now(),
		Locked:       false,
	}

	log.Printf("[DEBUG] ManualMarkAttendance: inserting new record for user %s", userID.Hex())
	if err := s.recordRepo.InsertRecord(ctx, record); err != nil {
		log.Printf("[ERROR] ManualMarkAttendance: InsertRecord error: %v", err)

		// Check for duplicate key error (unique index violation)
		if mongo.IsDuplicateKeyError(err) {
			log.Printf("[DEBUG] ManualMarkAttendance: duplicate key error, searching for existing record")
			// Try to find and update the existing record
			existing, findErr := s.recordRepo.FindRecord(ctx, filter)
			if findErr == nil && existing != nil {
				log.Printf("[DEBUG] ManualMarkAttendance: found existing record %s, updating", existing.ID.Hex())
				update := bson.M{
					"status":         status,
					"marked_by":      domain.MarkedByAdmin,
					"marked_by_user": markedBy,
					"submitted_at":   time.Now(),
					"deleted":        false,
					"deleted_at":     nil,
					"deleted_by":     "",
				}
				if updateErr := s.recordRepo.UpdateRecord(ctx, existing.ID, update); updateErr != nil {
					log.Printf("[ERROR] ManualMarkAttendance: UpdateRecord failed: %v", updateErr)
					return nil, updateErr
				}
				existing.Status = status
				existing.MarkedBy = domain.MarkedByAdmin
				existing.MarkedByUser = markedBy
				existing.Deleted = false
				return existing, nil
			}
			log.Printf("[DEBUG] ManualMarkAttendance: could not find existing record: %v", findErr)
		}

		// Check for other common errors
		if strings.Contains(err.Error(), "context deadline exceeded") {
			log.Printf("[ERROR] ManualMarkAttendance: database timeout")
			return nil, errors.New("database timeout, please try again")
		}

		return nil, err
	}

	log.Printf("[DEBUG] ManualMarkAttendance: success, record ID: %s", record.ID.Hex())
	return record, nil
}

func (s *SubmissionService) BulkMarkAttendance(userIDs []primitive.ObjectID, date string, session domain.AttendanceSession, status domain.AttendanceStatus, markedBy string) ([]domain.AttendanceRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var records []domain.AttendanceRecord

	for _, userID := range userIDs {
		user, err := s.userService.GetUserByID(userID.Hex())
		if err != nil {
			continue
		}

		filter := domain.AttendanceRecordFilter{
			UserID:  userID,
			Date:    date,
			Session: session,
		}

		existing, err := s.recordRepo.FindRecord(ctx, filter)
		isNotFound := errors.Is(err, ErrRecordNotFound) || (err != nil && err.Error() == "attendance record not found")
		if err != nil && !isNotFound {
			continue
		}

		if existing != nil && existing.ID != primitive.NilObjectID {
			update := bson.M{
				"status":         status,
				"marked_by":      domain.MarkedByAdmin,
				"marked_by_user": markedBy,
				"submitted_at":   time.Now(),
				"deleted":        false,
				"deleted_at":     nil,
				"deleted_by":     "",
			}
			if err := s.recordRepo.UpdateRecord(ctx, existing.ID, update); err != nil {
				continue
			}
			existing.Status = status
			existing.MarkedBy = domain.MarkedByAdmin
			existing.MarkedByUser = markedBy
			existing.Deleted = false
			records = append(records, *existing)
		} else {
			record := &domain.AttendanceRecord{
				UserID:       userID,
				JSDNumber:    user.JSDNumber,
				FirstName:    user.FirstName,
				LastName:     user.LastName,
				CohortNumber: user.CohortNumber,
				Date:         date,
				Session:      session,
				Status:       status,
				MarkedBy:     domain.MarkedByAdmin,
				MarkedByUser: markedBy,
				SubmittedAt:  time.Now(),
				Locked:       false,
			}

			if err := s.recordRepo.InsertRecord(ctx, record); err != nil {
				continue
			}
			records = append(records, *record)
		}
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

	filter := domain.AttendanceRecordFilter{}
	records, err := s.recordRepo.FindRecords(ctx, filter, nil)
	if err != nil {
		return nil, err
	}

	var record *domain.AttendanceRecord
	for _, r := range records {
		if r.ID == oid {
			record = &r
			break
		}
	}

	if record == nil {
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
		Date:    date,
		Session: domain.AttendanceSession(session),
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
		Date:    date,
		Session: domain.AttendanceSession(session),
		Cohort:  cohort,
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

	filter := domain.AttendanceRecordFilter{
		NotDeleted: true,
	}
	if cohort > 0 {
		filter.Cohort = cohort
	}
	if date != "" {
		filter.Date = date
	}

	records, err := s.recordRepo.FindRecords(ctx, filter, nil)
	if err != nil {
		return nil, 0, err
	}

	return records, len(records), nil
}

func (s *SubmissionService) GetStudentAttendanceHistory(userID primitive.ObjectID) ([]domain.AttendanceRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := domain.AttendanceRecordFilter{
		UserID:     userID,
		NotDeleted: true,
	}

	return s.recordRepo.FindRecords(ctx, filter, nil)
}

func (s *SubmissionService) GetUserAttendanceStatus(userID primitive.ObjectID) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pipeline := []bson.M{
		{"$match": bson.M{"user_id": userID}},
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
