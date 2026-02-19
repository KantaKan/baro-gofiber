package services

import (
	"context"
	"errors"
	"time"

	"gofiber-baro/config"
	"gofiber-baro/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	ErrLeaveRequestNotFound = errors.New("leave request not found")
	ErrInvalidLeaveType     = errors.New("invalid leave type")
	ErrInvalidSession       = errors.New("session is required for half_day type")
)

func CreateLeaveRequest(userID primitive.ObjectID, leaveType models.LeaveType, date string, session *models.AttendanceSession, reason string) (*models.LeaveRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user, err := GetUserByID(userID.Hex())
	if err != nil {
		return nil, ErrStudentNotFound
	}

	if leaveType != models.LeaveTypeLate && leaveType != models.LeaveTypeHalfDay && leaveType != models.LeaveTypeFullDay {
		return nil, ErrInvalidLeaveType
	}

	if leaveType == models.LeaveTypeHalfDay && session == nil {
		return nil, ErrInvalidSession
	}

	now := time.Now()
	request := models.LeaveRequest{
		UserID:        userID,
		JSDNumber:     user.JSDNumber,
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		CohortNumber:  user.CohortNumber,
		Type:          leaveType,
		Session:       session,
		Date:          date,
		Reason:        reason,
		Status:        models.LeaveStatusPending,
		CreatedAt:     now,
		CreatedBy:     "self",
		IsManualEntry: false,
	}

	result, err := config.LeaveRequestsCollection.InsertOne(ctx, request)
	if err != nil {
		return nil, err
	}

	request.ID = result.InsertedID.(primitive.ObjectID)
	return &request, nil
}

func AdminCreateLeaveRequest(userID primitive.ObjectID, leaveType models.LeaveType, date string, session *models.AttendanceSession, reason string, adminID primitive.ObjectID, adminName string) (*models.LeaveRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user, err := GetUserByID(userID.Hex())
	if err != nil {
		return nil, ErrStudentNotFound
	}

	if leaveType != models.LeaveTypeLate && leaveType != models.LeaveTypeHalfDay && leaveType != models.LeaveTypeFullDay {
		return nil, ErrInvalidLeaveType
	}

	if leaveType == models.LeaveTypeHalfDay && session == nil {
		return nil, ErrInvalidSession
	}

	now := time.Now()
	request := models.LeaveRequest{
		UserID:         userID,
		JSDNumber:      user.JSDNumber,
		FirstName:      user.FirstName,
		LastName:       user.LastName,
		CohortNumber:   user.CohortNumber,
		Type:           leaveType,
		Session:        session,
		Date:           date,
		Reason:         reason,
		Status:         models.LeaveStatusApproved,
		ReviewedBy:     &adminID,
		ReviewedByName: adminName,
		ReviewedAt:     &now,
		CreatedAt:      now,
		CreatedBy:      adminID.Hex(),
		IsManualEntry:  true,
	}

	result, err := config.LeaveRequestsCollection.InsertOne(ctx, request)
	if err != nil {
		return nil, err
	}

	request.ID = result.InsertedID.(primitive.ObjectID)

	err = autoMarkExcusedAttendance(userID, leaveType, date, session, adminID.Hex())
	if err != nil {
	}

	return &request, nil
}

func GetMyLeaveRequests(userID primitive.ObjectID) ([]models.LeaveRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := config.LeaveRequestsCollection.Find(ctx, bson.M{"user_id": userID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var requests []models.LeaveRequest
	if err := cursor.All(ctx, &requests); err != nil {
		return nil, err
	}

	return requests, nil
}

func GetAllLeaveRequests(cohort int, status string, fromDate string, toDate string) ([]models.LeaveRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{}
	if cohort > 0 {
		filter["cohort_number"] = cohort
	}
	if status != "" && status != "all" {
		filter["status"] = status
	}
	if fromDate != "" || toDate != "" {
		dateFilter := bson.M{}
		if fromDate != "" {
			dateFilter["$gte"] = fromDate
		}
		if toDate != "" {
			dateFilter["$lte"] = toDate
		}
		filter["date"] = dateFilter
	}

	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := config.LeaveRequestsCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var requests []models.LeaveRequest
	if err := cursor.All(ctx, &requests); err != nil {
		return nil, err
	}

	return requests, nil
}

func UpdateLeaveRequestStatus(requestID primitive.ObjectID, status models.LeaveRequestStatus, reviewNotes string, adminID primitive.ObjectID, adminName string) (*models.LeaveRequest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var request models.LeaveRequest
	err := config.LeaveRequestsCollection.FindOne(ctx, bson.M{"_id": requestID}).Decode(&request)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrLeaveRequestNotFound
		}
		return nil, err
	}

	if request.Status != models.LeaveStatusPending {
		return nil, errors.New("leave request already processed")
	}

	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"status":           status,
			"reviewed_by":      adminID,
			"reviewed_by_name": adminName,
			"reviewed_at":      now,
			"review_notes":     reviewNotes,
		},
	}

	_, err = config.LeaveRequestsCollection.UpdateOne(ctx, bson.M{"_id": requestID}, update)
	if err != nil {
		return nil, err
	}

	if status == models.LeaveStatusApproved {
		err = autoMarkExcusedAttendance(request.UserID, request.Type, request.Date, request.Session, adminID.Hex())
		if err != nil {
		}
	}

	request.Status = status
	request.ReviewedAt = &now
	request.ReviewNotes = reviewNotes
	request.ReviewedBy = &adminID
	request.ReviewedByName = adminName

	return &request, nil
}

func autoMarkExcusedAttendance(userID primitive.ObjectID, leaveType models.LeaveType, date string, session *models.AttendanceSession, markedBy string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user, err := GetUserByID(userID.Hex())
	if err != nil {
		return err
	}

	switch leaveType {
	case models.LeaveTypeLate:
		if session != nil {
			return markAttendanceStatus(ctx, userID, user, date, *session, models.StatusLateExcused, markedBy)
		}
		return markAttendanceStatus(ctx, userID, user, date, models.SessionMorning, models.StatusLateExcused, markedBy)

	case models.LeaveTypeHalfDay:
		if session != nil {
			return markAttendanceStatus(ctx, userID, user, date, *session, models.StatusAbsentExcused, markedBy)
		}
		return ErrInvalidSession

	case models.LeaveTypeFullDay:
		err1 := markAttendanceStatus(ctx, userID, user, date, models.SessionMorning, models.StatusAbsentExcused, markedBy)
		err2 := markAttendanceStatus(ctx, userID, user, date, models.SessionAfternoon, models.StatusAbsentExcused, markedBy)
		if err1 != nil {
			return err1
		}
		return err2
	}

	return nil
}

func markAttendanceStatus(ctx context.Context, userID primitive.ObjectID, user *models.User, date string, session models.AttendanceSession, status models.AttendanceStatus, markedBy string) error {
	filter := bson.M{
		"user_id": userID,
		"date":    date,
		"session": session,
	}

	var existing models.AttendanceRecord
	err := config.AttendanceRecordsCollection.FindOne(ctx, filter).Decode(&existing)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return err
	}

	if existing.ID != primitive.NilObjectID {
		update := bson.M{
			"$set": bson.M{
				"status":         status,
				"marked_by":      models.MarkedByAdmin,
				"marked_by_user": markedBy,
				"submitted_at":   time.Now(),
			},
		}
		_, err = config.AttendanceRecordsCollection.UpdateOne(ctx, filter, update)
		return err
	}

	record := models.AttendanceRecord{
		UserID:       userID,
		JSDNumber:    user.JSDNumber,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		CohortNumber: user.CohortNumber,
		Date:         date,
		Session:      session,
		Status:       status,
		MarkedBy:     models.MarkedByAdmin,
		MarkedByUser: markedBy,
		SubmittedAt:  time.Now(),
		Locked:       false,
	}

	_, err = config.AttendanceRecordsCollection.InsertOne(ctx, record)
	return err
}

func GetStudentAttendanceHistoryWithDays(userID primitive.ObjectID, days int) ([]models.AttendanceRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{
		"user_id": userID,
		"deleted": bson.M{"$ne": true},
	}

	if days > 0 {
		startDate := GetThailandTime().AddDate(0, 0, -days).Format("2006-01-02")
		filter["date"] = bson.M{"$gte": startDate}
	}

	opts := options.Find().SetSort(bson.D{{Key: "date", Value: -1}, {Key: "session", Value: 1}})
	cursor, err := config.AttendanceRecordsCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var records []models.AttendanceRecord
	if err := cursor.All(ctx, &records); err != nil {
		return nil, err
	}

	return records, nil
}

func GetStudentDailyStats(userID primitive.ObjectID, days int) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	startDate := GetThailandTime().AddDate(0, 0, -days).Format("2006-01-02")

	filter := bson.M{
		"user_id": userID,
		"deleted": bson.M{"$ne": true},
		"date":    bson.M{"$gte": startDate},
	}

	pipeline := []bson.M{
		{"$match": filter},
		{"$group": bson.M{
			"_id":            "$date",
			"present":        bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "present"}}, 1, 0}}},
			"late":           bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "late"}}, 1, 0}}},
			"absent":         bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "absent"}}, 1, 0}}},
			"late_excused":   bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "late_excused"}}, 1, 0}}},
			"absent_excused": bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "absent_excused"}}, 1, 0}}},
		}},
		{"$sort": bson.D{{Key: "_id", Value: 1}}},
	}

	cursor, err := config.AttendanceRecordsCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	stats := make([]map[string]interface{}, 0, len(results))
	for _, r := range results {
		date := r["_id"].(string)
		present := r["present"].(int32)
		late := r["late"].(int32)
		absent := r["absent"].(int32)
		lateExcused := r["late_excused"].(int32)
		absentExcused := r["absent_excused"].(int32)

		stats = append(stats, map[string]interface{}{
			"date":           date,
			"present":        present,
			"late":           late,
			"absent":         absent,
			"late_excused":   lateExcused,
			"absent_excused": absentExcused,
			"total":          present + late + absent + lateExcused + absentExcused,
		})
	}

	return stats, nil
}
