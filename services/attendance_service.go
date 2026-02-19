package services

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"gofiber-baro/config"
	"gofiber-baro/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Thailand timezone (UTC+7)
var thailandLocation *time.Location

func init() {
	thailandLocation, _ = time.LoadLocation("Asia/Bangkok")
}

// GetThailandTime returns current time in Thailand timezone
func GetThailandTime() time.Time {
	return time.Now().In(thailandLocation)
}

// GetThailandDate returns today's date string in Thailand timezone (YYYY-MM-DD)
func GetThailandDate() string {
	return GetThailandTime().Format("2006-01-02")
}

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

func GenerateCode(cohort int, session models.AttendanceSession, generatedBy string) (*models.AttendanceCode, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	code := generateRandomCode(string(session))

	now := GetThailandTime()
	expiresAt := now.Add(10 * time.Minute)

	fmt.Printf("Generating code: cohort=%d, session=%s, code=%s, expiresAt=%v\n", cohort, session, code, expiresAt)

	// First, deactivate old codes
	deactivateOldCodes(ctx, cohort, session)

	// Then insert the new code
	newCode := models.AttendanceCode{
		Code:         code,
		CohortNumber: cohort,
		Session:      session,
		GeneratedAt:  now,
		ExpiresAt:    expiresAt,
		IsActive:     true,
		GeneratedBy:  generatedBy,
	}

	_, err := config.AttendanceCodesCollection.InsertOne(ctx, newCode)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Code generated and saved: %+v\n", newCode)
	return &newCode, nil
}

func generateRandomCode(prefix string) string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	r := rand.New(rand.NewSource(GetThailandTime().UnixNano()))
	code := make([]byte, 4)
	for i := range code {
		code[i] = charset[r.Intn(len(charset))]
	}
	return strings.ToUpper(prefix) + "-" + string(code)
}

func deactivateOldCodes(ctx context.Context, cohort int, session models.AttendanceSession) {
	update := bson.M{
		"$set": bson.M{"is_active": false},
	}
	filter := bson.M{
		"cohort_number": cohort,
		"session":       session,
		"is_active":     true,
	}
	config.AttendanceCodesCollection.UpdateMany(ctx, filter, update)
}

func GetActiveCode(cohort int, session models.AttendanceSession) (*models.AttendanceCode, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var code models.AttendanceCode
	err := config.AttendanceCodesCollection.FindOne(ctx, bson.M{
		"cohort_number": cohort,
		"session":       string(session),
		"is_active":     true,
		"expires_at":    bson.M{"$gt": GetThailandTime()},
	}).Decode(&code)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}

	return &code, nil
}

func SubmitAttendance(userID primitive.ObjectID, code string, cohort int, ipAddress string) (*models.AttendanceRecord, error) {
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
	var session models.AttendanceSession
	if sessionStr == "morning" {
		session = models.SessionMorning
	} else if sessionStr == "afternoon" {
		session = models.SessionAfternoon
	} else {
		return nil, ErrInvalidCode
	}

	attendanceCode, err := GetActiveCode(cohort, session)
	if err != nil {
		return nil, err
	}

	if attendanceCode == nil {
		return nil, ErrNoActiveCode
	}

	if attendanceCode.Code != code {
		return nil, ErrInvalidCode
	}

	user, err := GetUserByID(userID.Hex())
	if err != nil {
		return nil, ErrStudentNotFound
	}

	if user.CohortNumber != cohort {
		return nil, ErrCodeForWrongCohort
	}

	today := GetThailandDate()

	existing, err := config.AttendanceRecordsCollection.CountDocuments(ctx, bson.M{
		"user_id": userID,
		"date":    today,
		"session": session,
		"deleted": bson.M{"$ne": true},
	})
	if err != nil {
		return nil, err
	}
	if existing > 0 {
		return nil, ErrAlreadySubmitted
	}

	isLocked, _ := IsSessionLocked(today, string(session), cohort)
	if isLocked {
		return nil, ErrSessionLocked
	}

	status := calculateStatus(session)

	record := models.AttendanceRecord{
		UserID:       userID,
		JSDNumber:    user.JSDNumber,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		CohortNumber: user.CohortNumber,
		Date:         today,
		Session:      session,
		Status:       status,
		MarkedBy:     models.MarkedBySelf,
		SubmittedAt:  time.Now(),
		Locked:       false,
		IPAddress:    ipAddress,
	}

	_, err = config.AttendanceRecordsCollection.InsertOne(ctx, record)
	if err != nil {
		return nil, err
	}

	return &record, nil
}

func calculateStatus(session models.AttendanceSession) models.AttendanceStatus {
	var startTime time.Time
	now := GetThailandTime()
	location := now.Location()

	if session == models.SessionMorning {
		startTime = time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, location)
	} else {
		startTime = time.Date(now.Year(), now.Month(), now.Day(), 13, 0, 0, 0, location)
	}

	elapsed := now.Sub(startTime)

	if elapsed <= 15*time.Minute {
		return models.StatusPresent
	} else if elapsed <= 90*time.Minute {
		return models.StatusLate
	} else {
		return models.StatusAbsent
	}
}

func GetUserAttendanceStatus(userID primitive.ObjectID) (map[string]interface{}, error) {
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

	cursor, err := config.AttendanceRecordsCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var result []bson.M
	if err := cursor.All(ctx, &result); err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"present":        0,
		"late":           0,
		"absent":         0,
		"late_excused":   0,
		"absent_excused": 0,
		"total_days":     0,
		"warning_level":  "normal",
	}

	if len(result) > 0 {
		stats["present"] = result[0]["present"]
		stats["late"] = result[0]["late"]
		stats["absent"] = result[0]["absent"]
		stats["late_excused"] = result[0]["late_excused"]
		stats["absent_excused"] = result[0]["absent_excused"]
		stats["total_days"] = result[0]["total_days"]

		absent := result[0]["absent"].(int32)
		if absent >= 7 {
			stats["warning_level"] = "red"
		} else if absent >= 4 {
			stats["warning_level"] = "yellow"
		} else {
			stats["warning_level"] = "normal"
		}
	}

	return stats, nil
}

func ManualMarkAttendance(userID primitive.ObjectID, date, session string, status models.AttendanceStatus, markedBy string) (*models.AttendanceRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user, err := GetUserByID(userID.Hex())
	if err != nil {
		return nil, ErrStudentNotFound
	}

	filter := bson.M{
		"user_id": userID,
		"date":    date,
		"session": session,
	}

	var existing models.AttendanceRecord
	err = config.AttendanceRecordsCollection.FindOne(ctx, filter).Decode(&existing)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	if existing.ID != primitive.NilObjectID {
		update := bson.M{
			"$set": bson.M{
				"status":         status,
				"marked_by":      models.MarkedByAdmin,
				"marked_by_user": markedBy,
				"submitted_at":   time.Now(),
				"deleted":        false,
				"deleted_at":     nil,
				"deleted_by":     "",
			},
		}
		_, err = config.AttendanceRecordsCollection.UpdateOne(ctx, filter, update)
		if err != nil {
			return nil, err
		}
		existing.Status = status
		existing.MarkedBy = models.MarkedByAdmin
		existing.MarkedByUser = markedBy
		existing.Deleted = false
		return &existing, nil
	}

	record := models.AttendanceRecord{
		UserID:       userID,
		JSDNumber:    user.JSDNumber,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		CohortNumber: user.CohortNumber,
		Date:         date,
		Session:      models.AttendanceSession(session),
		Status:       status,
		MarkedBy:     models.MarkedByAdmin,
		MarkedByUser: markedBy,
		SubmittedAt:  time.Now(),
		Locked:       false,
	}

	_, err = config.AttendanceRecordsCollection.InsertOne(ctx, record)
	if err != nil {
		return nil, err
	}

	return &record, nil
}

func GetAttendanceLogs(cohort int, date string, page, limit int) ([]models.AttendanceRecord, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{
		"deleted": bson.M{"$ne": true},
	}
	if cohort > 0 {
		filter["cohort_number"] = cohort
	}
	if date != "" {
		filter["date"] = date
	}

	total, err := config.AttendanceRecordsCollection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	skip := int64((page - 1) * limit)
	limitInt64 := int64(limit)
	cursor, err := config.AttendanceRecordsCollection.Find(ctx, filter, &options.FindOptions{
		Sort:  bson.D{{Key: "submitted_at", Value: -1}},
		Skip:  &skip,
		Limit: &limitInt64,
	})
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var records []models.AttendanceRecord
	if err := cursor.All(ctx, &records); err != nil {
		return nil, 0, err
	}

	return records, int(total), nil
}

func GetAttendanceStats(cohort int) ([]models.AttendanceStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{
		"deleted": bson.M{"$ne": true},
	}
	if cohort > 0 {
		filter["cohort_number"] = cohort
	}

	// Get all records and calculate in Go for simpler logic
	cursor, err := config.AttendanceRecordsCollection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var records []models.AttendanceRecord
	if err := cursor.All(ctx, &records); err != nil {
		return nil, err
	}

	// Get all holidays to exclude from stats
	holidays, _ := GetHolidays()
	holidayDates := make(map[string]bool)
	for _, h := range holidays {
		start, err := time.Parse("2006-01-02", h.StartDate)
		if err != nil {
			continue
		}
		end, err := time.Parse("2006-01-02", h.EndDate)
		if err != nil {
			continue
		}
		for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
			holidayDates[d.Format("2006-01-02")] = true
		}
	}

	// Group by user and date
	type dayKey struct {
		userID       string
		jsdNumber    string
		firstName    string
		lastName     string
		cohortNumber int
		date         string
	}

	type userKey struct {
		userID       string
		jsdNumber    string
		firstName    string
		lastName     string
		cohortNumber int
	}

	dayMap := make(map[dayKey]map[string]bool)
	for _, r := range records {
		if r.Deleted {
			continue
		}
		// Skip holidays
		if holidayDates[r.Date] {
			continue
		}
		key := dayKey{
			userID:       r.UserID.Hex(),
			jsdNumber:    r.JSDNumber,
			firstName:    r.FirstName,
			lastName:     r.LastName,
			cohortNumber: r.CohortNumber,
			date:         r.Date,
		}
		if dayMap[key] == nil {
			dayMap[key] = make(map[string]bool)
		}
		dayMap[key][string(r.Session)] = true
	}

	// Calculate stats per user
	userStats := make(map[userKey]int)
	absentDays := make(map[userKey]int)
	for key, sessions := range dayMap {
		uk := userKey{
			userID:       key.userID,
			jsdNumber:    key.jsdNumber,
			firstName:    key.firstName,
			lastName:     key.lastName,
			cohortNumber: key.cohortNumber,
		}
		// Present if both morning and afternoon attended
		if sessions["morning"] && sessions["afternoon"] {
			userStats[uk]++
		} else {
			absentDays[uk]++
		}
	}

	// Convert to response
	stats := make([]models.AttendanceStats, 0, len(userStats))
	for uk, present := range userStats {
		absent := absentDays[uk]
		totalDays := present + absent
		userID, _ := primitive.ObjectIDFromHex(uk.userID)

		warningLevel := "normal"
		if absent >= 7 {
			warningLevel = "red"
		} else if absent >= 4 {
			warningLevel = "yellow"
		}

		stats = append(stats, models.AttendanceStats{
			UserID:        userID,
			JSDNumber:     uk.jsdNumber,
			FirstName:     uk.firstName,
			LastName:      uk.lastName,
			CohortNumber:  uk.cohortNumber,
			Present:       present,
			Late:          0,
			LateExcused:   0,
			Absent:        absent,
			AbsentExcused: 0,
			TotalDays:     totalDays,
			WarningLevel:  warningLevel,
		})
	}

	return stats, nil
}

func GetStudentAttendanceHistory(userID primitive.ObjectID) ([]models.AttendanceRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := config.AttendanceRecordsCollection.Find(ctx, bson.M{
		"user_id": userID,
	}, &options.FindOptions{
		Sort: bson.D{{Key: "date", Value: -1}, {Key: "session", Value: 1}},
	})
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

func GetTodayAttendanceOverview(cohort int, session models.AttendanceSession) (*models.TodayAttendanceOverview, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	today := GetThailandDate()

	var activeCode *models.AttendanceCode
	// Only get active code if session is specified
	if session != "" {
		activeCode, _ = GetActiveCode(cohort, session)
	}

	filter := bson.M{
		"cohort_number": cohort,
		"date":          today,
		"deleted":       bson.M{"$ne": true},
	}
	if session != "" {
		filter["session"] = session
	}

	cursor, err := config.AttendanceRecordsCollection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var records []models.AttendanceRecord
	if err := cursor.All(ctx, &records); err != nil {
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

	users, _, err := GetAllUsers(cohort, "", "", "", "first_name", 1, 1, 500)
	if err != nil {
		return nil, err
	}

	students := make([]models.StudentAttendanceRow, 0, len(users))
	for _, user := range users {
		row := models.StudentAttendanceRow{
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

	overview := &models.TodayAttendanceOverview{
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

func GetAttendanceOverviewByDate(cohort int, session models.AttendanceSession, date string) (*models.TodayAttendanceOverview, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	targetDate := date
	if targetDate == "" {
		targetDate = GetThailandDate()
	}

	var activeCode *models.AttendanceCode
	if session != "" && targetDate == GetThailandDate() {
		activeCode, _ = GetActiveCode(cohort, session)
	}

	filter := bson.M{
		"cohort_number": cohort,
		"date":          targetDate,
		"deleted":       bson.M{"$ne": true},
	}
	if session != "" {
		filter["session"] = session
	}

	cursor, err := config.AttendanceRecordsCollection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var records []models.AttendanceRecord
	if err := cursor.All(ctx, &records); err != nil {
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

	users, _, err := GetAllUsers(cohort, "", "", "", "first_name", 1, 1, 500)
	if err != nil {
		return nil, err
	}

	students := make([]models.StudentAttendanceRow, 0, len(users))
	for _, user := range users {
		row := models.StudentAttendanceRow{
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

	overview := &models.TodayAttendanceOverview{
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

func LockSession(date, session string, cohort int, locked bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{
		"date":    date,
		"session": session,
	}
	if cohort > 0 {
		filter["cohort_number"] = cohort
	}

	update := bson.M{
		"$set": bson.M{"locked": locked},
	}

	_, err := config.AttendanceRecordsCollection.UpdateMany(ctx, filter, update)
	return err
}

func IsSessionLocked(date, session string, cohort int) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{
		"date":          date,
		"session":       session,
		"cohort_number": cohort,
		"locked":        true,
	}

	count, err := config.AttendanceRecordsCollection.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func DeleteAttendanceRecord(recordID string, deletedBy string) (*models.AttendanceRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	oid, err := primitive.ObjectIDFromHex(recordID)
	if err != nil {
		return nil, ErrRecordNotFound
	}

	filter := bson.M{"_id": oid}
	update := bson.M{
		"$set": bson.M{
			"deleted":    true,
			"deleted_at": time.Now(),
			"deleted_by": deletedBy,
		},
	}

	_, err = config.AttendanceRecordsCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	var record models.AttendanceRecord
	err = config.AttendanceRecordsCollection.FindOne(ctx, filter).Decode(&record)
	if err != nil {
		return nil, ErrRecordNotFound
	}

	return &record, nil
}

func GetAttendanceStatsWithFilter(cohort int, days int) ([]models.AttendanceStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	startDate := GetThailandTime().AddDate(0, 0, -days).Format("2006-01-02")

	filter := bson.M{
		"deleted": bson.M{"$ne": true},
	}
	if cohort > 0 {
		filter["cohort_number"] = cohort
	}
	if days > 0 {
		filter["date"] = bson.M{"$gte": startDate}
	}

	pipeline := []bson.M{
		{"$match": filter},
		{"$group": bson.M{
			"_id": bson.M{
				"user_id":       "$user_id",
				"jsd_number":    "$jsd_number",
				"first_name":    "$first_name",
				"last_name":     "$last_name",
				"cohort_number": "$cohort_number",
			},
			"present":        bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "present"}}, 1, 0}}},
			"late":           bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "late"}}, 1, 0}}},
			"absent":         bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "absent"}}, 1, 0}}},
			"late_excused":   bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "late_excused"}}, 1, 0}}},
			"absent_excused": bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "absent_excused"}}, 1, 0}}},
		}},
		{"$sort": bson.D{{Key: "absent", Value: -1}}},
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

	stats := make([]models.AttendanceStats, 0, len(results))
	for _, r := range results {
		id := r["_id"].(bson.M)
		absent := r["absent"].(int32)

		warningLevel := "normal"
		if absent >= 7 {
			warningLevel = "red"
		} else if absent >= 4 {
			warningLevel = "yellow"
		}

		stat := models.AttendanceStats{
			UserID:        id["user_id"].(primitive.ObjectID),
			JSDNumber:     id["jsd_number"].(string),
			FirstName:     id["first_name"].(string),
			LastName:      id["last_name"].(string),
			CohortNumber:  int(id["cohort_number"].(int32)),
			Present:       int(r["present"].(int32)),
			Late:          int(r["late"].(int32)),
			LateExcused:   int(r["late_excused"].(int32)),
			Absent:        int(absent),
			AbsentExcused: int(r["absent_excused"].(int32)),
			WarningLevel:  warningLevel,
		}
		stat.TotalDays = stat.Present + stat.Late + stat.Absent + stat.LateExcused + stat.AbsentExcused
		stats = append(stats, stat)
	}

	return stats, nil
}

func GetDailyAttendanceStats(cohort int, days int) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	startDate := GetThailandTime().AddDate(0, 0, -days).Format("2006-01-02")

	filter := bson.M{
		"deleted": bson.M{"$ne": true},
	}
	if cohort > 0 {
		filter["cohort_number"] = cohort
	}
	if days > 0 {
		filter["date"] = bson.M{"$gte": startDate}
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
		stats = append(stats, map[string]interface{}{
			"date":           date,
			"present":        r["present"],
			"late":           r["late"],
			"absent":         r["absent"],
			"late_excused":   r["late_excused"],
			"absent_excused": r["absent_excused"],
			"total":          r["present"].(int32) + r["late"].(int32) + r["absent"].(int32) + r["late_excused"].(int32) + r["absent_excused"].(int32),
		})
	}

	return stats, nil
}

func BulkMarkAttendance(userIDs []primitive.ObjectID, date string, session models.AttendanceSession, status models.AttendanceStatus, markedBy string) ([]models.AttendanceRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var records []models.AttendanceRecord

	for _, userID := range userIDs {
		user, err := GetUserByID(userID.Hex())
		if err != nil {
			continue
		}

		filter := bson.M{
			"user_id": userID,
			"date":    date,
			"session": session,
		}

		var existing models.AttendanceRecord
		err = config.AttendanceRecordsCollection.FindOne(ctx, filter).Decode(&existing)
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			continue
		}

		if existing.ID != primitive.NilObjectID {
			update := bson.M{
				"$set": bson.M{
					"status":         status,
					"marked_by":      models.MarkedByAdmin,
					"marked_by_user": markedBy,
					"submitted_at":   time.Now(),
					"deleted":        false,
					"deleted_at":     nil,
					"deleted_by":     "",
				},
			}
			_, err = config.AttendanceRecordsCollection.UpdateOne(ctx, filter, update)
			if err != nil {
				continue
			}
			existing.Status = status
			existing.MarkedBy = models.MarkedByAdmin
			existing.MarkedByUser = markedBy
			existing.Deleted = false
			records = append(records, existing)
		} else {
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
			if err != nil {
				continue
			}
			records = append(records, record)
		}
	}

	return records, nil
}
