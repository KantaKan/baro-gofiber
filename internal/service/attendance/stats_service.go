package attendance

import (
	"context"
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

	filter := domain.AttendanceRecordFilter{
		NotDeleted: true,
	}

	if cohort > 0 {
		filter.Cohort = cohort
	}

	records, err := s.recordRepo.FindRecords(ctx, filter, nil)
	if err != nil {
		return nil, err
	}

	type userKey struct {
		userID       string
		jsdNumber    string
		firstName    string
		lastName     string
		cohortNumber int
	}

	type userStats struct {
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

	userStatsMap := make(map[userKey]*userStats)

	for _, r := range records {
		if startDate != "" && endDate != "" {
			if r.Date < startDate || r.Date > endDate {
				continue
			}
		}

		uk := userKey{
			userID:       r.UserID.Hex(),
			jsdNumber:    r.JSDNumber,
			firstName:    r.FirstName,
			lastName:     r.LastName,
			cohortNumber: r.CohortNumber,
		}

		if userStatsMap[uk] == nil {
			userStatsMap[uk] = &userStats{
				dates: make(map[string]struct {
					morning   string
					afternoon string
				}),
			}
		}

		status := string(r.Status)
		switch status {
		case "present":
			userStatsMap[uk].present++
		case "late":
			userStatsMap[uk].late++
		case "absent":
			userStatsMap[uk].absent++
		case "late_excused":
			userStatsMap[uk].lateExcused++
		case "absent_excused":
			userStatsMap[uk].absentExcused++
		}

		dateData := userStatsMap[uk].dates[r.Date]
		if r.Session == "morning" {
			dateData.morning = status
		} else {
			dateData.afternoon = status
		}
		userStatsMap[uk].dates[r.Date] = dateData
	}

	stats := make([]domain.AttendanceStats, 0, len(userStatsMap))
	for uk, us := range userStatsMap {
		userID, _ := primitive.ObjectIDFromHex(uk.userID)

		presentDays := 0
		absentDays := 0
		for _, dateData := range us.dates {
			hasAbsent := dateData.morning == "absent" || dateData.afternoon == "absent"
			hasPresent := dateData.morning == "present" || dateData.afternoon == "present"
			hasLate := dateData.morning == "late" || dateData.afternoon == "late"
			hasLateExcused := dateData.morning == "late_excused" || dateData.afternoon == "late_excused"

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
			JSDNumber:     uk.jsdNumber,
			FirstName:     uk.firstName,
			LastName:      uk.lastName,
			CohortNumber:  uk.cohortNumber,
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

func (s *StatsService) GetAttendanceStatsWithFilter(cohort int, days int) ([]domain.AttendanceStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	startDate := utils.GetThailandTime().AddDate(0, 0, -days).Format("2006-01-02")

	pipeline := []bson.M{
		{"$match": bson.M{
			"deleted": bson.M{"$ne": true},
			"date":    bson.M{"$gte": startDate},
		}},
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

	return s.recordRepo.AggregateStats(ctx, pipeline)
}

func (s *StatsService) GetDailyAttendanceStats(cohort int, days int) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	startDate := utils.GetThailandTime().AddDate(0, 0, -days).Format("2006-01-02")

	pipeline := []bson.M{
		{"$match": bson.M{
			"deleted": bson.M{"$ne": true},
			"date":    bson.M{"$gte": startDate},
		}},
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

	return s.recordRepo.AggregateDailyStats(ctx, pipeline)
}
