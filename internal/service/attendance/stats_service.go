package attendance

import (
	"context"
	"log"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type StatsService struct {
	recordRepo  domain.AttendanceRepository
	userService UserServiceInterface
}

func NewStatsService(recordRepo domain.AttendanceRepository, userService UserServiceInterface) *StatsService {
	return &StatsService{
		recordRepo:  recordRepo,
		userService: userService,
	}
}

func (s *StatsService) GetAttendanceStats(cohort int, startDate, endDate string) ([]domain.AttendanceStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	bsonFilter := bson.M{
		"deleted": bson.M{"$ne": true},
	}
	if cohort > 0 {
		bsonFilter["cohort_number"] = cohort
	}
	if startDate != "" && endDate != "" {
		bsonFilter["date"] = bson.M{"$gte": startDate, "$lte": endDate}
	}

	records, err := s.recordRepo.FindRecordsRaw(ctx, bsonFilter, nil)
	if err != nil {
		return nil, err
	}

	// Group by user_id so a name edit never splits one learner into two rows.
	type sessKey struct {
		date    string
		session string
	}
	type userStats struct {
		userID        string
		jsdNumber     string
		firstName     string
		lastName      string
		cohortNumber  int
		present       int
		late          int
		absent        int
		lateExcused   int
		absentExcused int
		dates         map[string]struct {
			morning   string
			afternoon string
		}
	}

	userMap := make(map[string]*userStats)
	countedSessions := make(map[string]bool) // "userID-date-session"

	for _, r := range records {
		uid := r.UserID.Hex()
		sk := uid + "-" + r.Date + "-" + string(r.Session)
		if countedSessions[sk] {
			continue
		}
		countedSessions[sk] = true

		us, ok := userMap[uid]
		if !ok {
			us = &userStats{
				userID:       uid,
				jsdNumber:    r.JSDNumber,
				firstName:    r.FirstName,
				lastName:     r.LastName,
				cohortNumber: r.CohortNumber,
				dates:        make(map[string]struct{ morning, afternoon string }),
			}
			userMap[uid] = us
		}

		status := string(r.Status)
		switch status {
		case "present":
			us.present++
		case "late":
			us.late++
		case "absent":
			us.absent++
		case "late_excused":
			us.lateExcused++
		case "absent_excused":
			us.absentExcused++
		}

		dd := us.dates[r.Date]
		if r.Session == "morning" {
			dd.morning = status
		} else {
			dd.afternoon = status
		}
		us.dates[r.Date] = dd
	}

	stats := make([]domain.AttendanceStats, 0, len(userMap))
	for _, us := range userMap {
		userID, _ := primitive.ObjectIDFromHex(us.userID)

		presentDays := 0
		absentDays := 0
		for _, dd := range us.dates {
			hasAbsent := dd.morning == "absent" || dd.afternoon == "absent"
			hasPresent := dd.morning == "present" || dd.afternoon == "present"
			hasLate := dd.morning == "late" || dd.afternoon == "late"
			hasLateExcused := dd.morning == "late_excused" || dd.afternoon == "late_excused"

			if hasAbsent {
				absentDays++
			} else if hasPresent || hasLate || hasLateExcused {
				presentDays++
			}
		}

		warningLevel := "normal"
		if us.absent >= 7 {
			warningLevel = "red"
		} else if us.absent >= 4 {
			warningLevel = "yellow"
		}

		stats = append(stats, domain.AttendanceStats{
			UserID:        userID,
			JSDNumber:     us.jsdNumber,
			FirstName:     us.firstName,
			LastName:      us.lastName,
			CohortNumber:  us.cohortNumber,
			Present:       us.present,
			Late:          us.late,
			LateExcused:   us.lateExcused,
			Absent:        us.absent,
			AbsentExcused: us.absentExcused,
			PresentDays:   presentDays,
			AbsentDays:    absentDays,
			WarningLevel:  warningLevel,
		})
	}

	return stats, nil
}

// toInt - helper to safely convert various numeric types from MongoDB to int
func toInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch i := v.(type) {
	case int:
		return i
	case int32:
		return int(i)
	case int64:
		return int(i)
	case float64:
		return int(i)
	default:
		return 0
	}
}

// GetDailyAttendanceStatsByDateRange - get daily stats for a specific date range
func (s *StatsService) GetDailyAttendanceStatsByDateRange(cohort int, startDate, endDate string) ([]map[string]interface{}, error) {
	// If no startDate provided, default to 30 days ago
	if startDate == "" {
		startDate = utils.GetThailandTime().AddDate(0, 0, -30).Format("2006-01-02")
	}
	if endDate == "" {
		endDate = utils.GetThailandTime().Format("2006-01-02")
	}

	log.Printf("[DEBUG] GetDailyAttendanceStatsByDateRange: cohort=%d, startDate=%s, endDate=%s", cohort, startDate, endDate)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build match filter with cohort if provided
	matchFilter := bson.M{
		"deleted": bson.M{"$ne": true},
		"date":    bson.M{"$gte": startDate, "$lte": endDate},
	}
	if cohort > 0 {
		matchFilter["cohort_number"] = cohort
	}

	// First aggregation: get stats grouped by date AND session (AM/PM)
	// We first group by date, session and user_id to ensure each student is only counted once per session
	pipeline := []bson.M{
		{"$match": matchFilter},
		{"$group": bson.M{
			"_id": bson.M{
				"date":    "$date",
				"session": "$session",
				"user_id": "$user_id",
			},
			"status": bson.M{"$first": "$status"},
		}},
		{"$group": bson.M{
			"_id": bson.M{
				"date":    "$_id.date",
				"session": "$_id.session",
			},
			"present":        bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "present"}}, 1, 0}}},
			"late":           bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "late"}}, 1, 0}}},
			"absent":         bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "absent"}}, 1, 0}}},
			"late_excused":   bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "late_excused"}}, 1, 0}}},
			"absent_excused": bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "absent_excused"}}, 1, 0}}},
		}},
		{"$sort": bson.D{{Key: "_id.date", Value: 1}, {Key: "_id.session", Value: 1}}},
	}

	results, err := s.recordRepo.AggregateDailyStats(ctx, pipeline)
	if err != nil {
		log.Printf("[ERROR] GetDailyAttendanceStats aggregation failed: %v", err)
		return nil, err
	}

	// Get cohort learner count
	cohortTotal := 0
	if cohort > 0 {
		users, _, err := s.userService.GetAllUsers(cohort, "learner", "", "", "email", 1, 0, 0, "dropout,dismissed")
		if err == nil {
			cohortTotal = len(users)
		} else {
			log.Printf("[WARN] GetDailyAttendanceStats: could not get cohort %d count: %v", cohort, err)
		}
	}

	// Transform results: group by date, combine AM/PM
	dateMap := make(map[string]map[string]interface{})
	for _, r := range results {
		var date, session string

		// Robustly extract date and session from _id or top-level
		if id, ok := r["_id"]; ok {
			if idMap, ok := id.(map[string]interface{}); ok {
				date, _ = idMap["date"].(string)
				session, _ = idMap["session"].(string)
			} else if idMap, ok := id.(primitive.M); ok {
				date, _ = idMap["date"].(string)
				session, _ = idMap["session"].(string)
			} else if idD, ok := id.(primitive.D); ok {
				for _, e := range idD {
					if e.Key == "date" {
						date, _ = e.Value.(string)
					} else if e.Key == "session" {
						session, _ = e.Value.(string)
					}
				}
			}
		}

		// Fallback to top-level if not in _id
		if date == "" {
			date, _ = r["date"].(string)
		}
		if session == "" {
			session, _ = r["session"].(string)
		}

		if date == "" {
			log.Printf("[WARN] GetDailyAttendanceStats: missing date in result: %v", r)
			continue
		}

		if _, exists := dateMap[date]; !exists {
			dateMap[date] = map[string]interface{}{
				"date":           date,
				"present":        0,
				"late":           0,
				"absent":         0,
				"late_excused":   0,
				"absent_excused": 0,
				"am_present":     0,
				"pm_present":     0,
				"am_late":        0,
				"pm_late":        0,
				"am_total":       0,
				"pm_total":       0,
			}
		}

		// Add to totals using the safe toInt helper
		present := toInt(r["present"])
		late := toInt(r["late"])
		lateExcused := toInt(r["late_excused"])
		absentExcused := toInt(r["absent_excused"])
		absent := toInt(r["absent"])

		day := dateMap[date]
		day["present"] = day["present"].(int) + present
		day["late"] = day["late"].(int) + late
		day["late_excused"] = day["late_excused"].(int) + lateExcused
		day["absent_excused"] = day["absent_excused"].(int) + absentExcused
		day["absent"] = day["absent"].(int) + absent

		// Session-specific counts
		sessionPresent := present + late + lateExcused
		sessionAbsentTotal := absent + absentExcused

		// Use cohortTotal for denominators if available
		totalForSession := cohortTotal
		if totalForSession == 0 {
			totalForSession = sessionPresent + sessionAbsentTotal
		}

		if session == "morning" {
			day["am_present"] = day["am_present"].(int) + sessionPresent
			day["am_late"] = day["am_late"].(int) + late
			day["am_total"] = totalForSession
		} else if session == "afternoon" {
			day["pm_present"] = day["pm_present"].(int) + sessionPresent
			day["pm_late"] = day["pm_late"].(int) + late
			day["pm_total"] = totalForSession
		}
	}

	// Convert map to slice and add cohort total
	finalResults := make([]map[string]interface{}, 0, len(dateMap))
	for _, v := range dateMap {
		v["total"] = cohortTotal

		// Calculate attendance rate
		// We use (am_present + pm_present) / (cohortTotal * 2)
		// If cohortTotal is 0, we fallback to the sum of records
		presentSum := v["present"].(int)
		lateSum := v["late"].(int)
		lateExcusedSum := v["late_excused"].(int)

		attended := float64(presentSum + lateSum + lateExcusedSum)
		var totalPossible float64

		if cohortTotal > 0 {
			totalPossible = float64(cohortTotal * 2)
		} else {
			absentSum := v["absent"].(int)
			absentExcusedSum := v["absent_excused"].(int)
			totalPossible = float64(presentSum + lateSum + lateExcusedSum + absentSum + absentExcusedSum)
		}

		rate := 0.0
		if totalPossible > 0 {
			rate = (attended / totalPossible) * 100
		}
		v["rate"] = rate
		finalResults = append(finalResults, v)
	}

	log.Printf("[DEBUG] GetDailyAttendanceStatsByDateRange: cohort=%d, startDate=%s, endDate=%s, totalDates=%d, cohortTotal=%d", cohort, startDate, endDate, len(finalResults), cohortTotal)

	return finalResults, nil
}
