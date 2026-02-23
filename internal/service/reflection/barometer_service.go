package reflection

import (
	"context"
	"strings"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type BarometerData struct {
	Date                             string `json:"date"`
	ComfortZone                      int    `json:"Comfort Zone"`
	PanicZone                        int    `json:"Panic Zone"`
	StretchZoneEnjoyingTheChallenges int    `json:"Stretch Zone - Enjoying the Challenges"`
	StretchZoneOverwhelmed           int    `json:"Stretch Zone - Overwhelmed"`
}

type BarometerService struct {
	db *mongo.Database
}

func NewBarometerService(db *mongo.Database) *BarometerService {
	return &BarometerService{db: db}
}

func (s *BarometerService) GetUserBarometerData(users []domain.User) (map[string]int, error) {
	zoneCounts := map[string]int{
		"Comfort Zone":                           0,
		"Stretch Zone - Enjoying the Challenges": 0,
		"Stretch Zone - Overwhelmed":             0,
		"Panic Zone":                             0,
	}

	for _, user := range users {
		for _, reflection := range user.Reflections {
			barometer := reflection.ReflectionData.Barometer

			switch strings.ToLower(barometer) {
			case "comfort zone":
				zoneCounts["Comfort Zone"]++
			case "stretch zone- enjoying the challenges":
				zoneCounts["Stretch Zone - Enjoying the Challenges"]++
			case "stretch zone - overwhelmed":
				zoneCounts["Stretch Zone - Overwhelmed"]++
			case "panic zone":
				zoneCounts["Panic Zone"]++
			}
		}
	}

	return zoneCounts, nil
}

func (s *BarometerService) GetAllUsersBarometerData(timeRange string, cohort int) ([]BarometerData, error) {
	ctx := context.Background()

	endDate := time.Now()
	startDate := time.Now()

	switch timeRange {
	case "90d":
		startDate = endDate.AddDate(0, 0, -90)
	case "30d":
		startDate = endDate.AddDate(0, 0, -30)
	case "7d":
		startDate = endDate.AddDate(0, 0, -7)
	default:
		startDate = endDate.AddDate(0, 0, -90)
	}

	matchFilter := bson.M{
		"reflections.date": bson.M{
			"$gte": startDate,
			"$lte": endDate,
		},
	}
	if cohort > 0 {
		matchFilter["cohort_number"] = cohort
	}

	pipeline := []bson.M{
		{"$unwind": "$reflections"},
		{"$match": matchFilter},
		{"$group": bson.M{
			"_id": bson.M{
				"date": bson.M{
					"$dateToString": bson.M{
						"format": "%Y-%m-%d",
						"date":   "$reflections.date",
					},
				},
				"barometer": "$reflections.reflection.barometer",
			},
			"count": bson.M{"$sum": 1},
		}},
		{"$sort": bson.M{"_id.date": 1}},
	}

	cursor, err := s.db.Collection("users").Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	type aggregateResult struct {
		ID struct {
			Date      string `bson:"date"`
			Barometer string `bson:"barometer"`
		} `bson:"_id"`
		Count int `bson:"count"`
	}

	var results []aggregateResult
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	dataMap := make(map[string]*BarometerData)

	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		dataMap[dateStr] = &BarometerData{
			Date:                             dateStr,
			ComfortZone:                      0,
			PanicZone:                        0,
			StretchZoneEnjoyingTheChallenges: 0,
			StretchZoneOverwhelmed:           0,
		}
	}

	for _, result := range results {
		data, exists := dataMap[result.ID.Date]
		if !exists {
			continue
		}

		switch result.ID.Barometer {
		case "Comfort Zone":
			data.ComfortZone = result.Count
		case "Panic Zone":
			data.PanicZone = result.Count
		case "Stretch zone - Enjoying the challenges":
			data.StretchZoneEnjoyingTheChallenges = result.Count
		case "Stretch zone - Overwhelmed":
			data.StretchZoneOverwhelmed = result.Count
		}
	}

	var chartData []BarometerData
	for _, data := range dataMap {
		chartData = append(chartData, *data)
	}

	return chartData, nil
}

func (s *BarometerService) GetWeeklySummary(page, limit, cohort int) ([]domain.WeeklySummary, int, error) {
	ctx := context.Background()

	collection := s.db.Collection("users")

	matchFilter := bson.M{
		"reflections.reflection.barometer": bson.M{
			"$in": bson.A{"Stretch zone - Overwhelmed", "Panic Zone"},
		},
	}
	if cohort > 0 {
		matchFilter["cohort_number"] = cohort
	}

	unwindStage := bson.D{{Key: "$unwind", Value: "$reflections"}}
	matchStage := bson.D{{Key: "$match", Value: matchFilter}}
	groupStage := bson.D{{Key: "$group", Value: bson.D{
		{Key: "_id", Value: bson.D{
			{Key: "year", Value: bson.D{{Key: "$year", Value: "$reflections.date"}}},
			{Key: "week", Value: bson.D{{Key: "$isoWeek", Value: "$reflections.date"}}},
		}},
	}}}
	countStage := bson.D{{Key: "$count", Value: "total"}}

	countPipeline := mongo.Pipeline{unwindStage, matchStage, groupStage, countStage}
	countCursor, err := collection.Aggregate(ctx, countPipeline)
	if err != nil {
		return nil, 0, err
	}
	var countResult []bson.M
	if err = countCursor.All(ctx, &countResult); err != nil {
		return nil, 0, err
	}
	total := 0
	if len(countResult) > 0 {
		total = int(countResult[0]["total"].(int32))
	}
	countCursor.Close(ctx)

	paginatedPipeline := mongo.Pipeline{
		unwindStage,
		matchStage,
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "year", Value: bson.D{{Key: "$year", Value: "$reflections.date"}}},
				{Key: "week", Value: bson.D{{Key: "$isoWeek", Value: "$reflections.date"}}},
			}},
			{Key: "students", Value: bson.D{{Key: "$push", Value: bson.D{
				{Key: "user_id", Value: bson.D{{Key: "$toString", Value: "$_id"}}},
				{Key: "first_name", Value: "$first_name"},
				{Key: "last_name", Value: "$last_name"},
				{Key: "zoom_name", Value: "$zoom_name"},
				{Key: "jsd_number", Value: "$jsd_number"},
				{Key: "barometer", Value: "$reflections.reflection.barometer"},
				{Key: "date", Value: "$reflections.date"},
			}}}},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{
			{Key: "_id.year", Value: -1},
			{Key: "_id.week", Value: -1},
		}}},
		bson.D{{Key: "$skip", Value: (page - 1) * limit}},
		bson.D{{Key: "$limit", Value: limit}},
	}

	cursor, err := collection.Aggregate(ctx, paginatedPipeline)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var results []struct {
		ID struct {
			Year int `bson:"year"`
			Week int `bson:"week"`
		} `bson:"_id"`
		Students []domain.StudentInfo `bson:"students"`
	}

	if err = cursor.All(ctx, &results); err != nil {
		return nil, 0, err
	}

	var weeklySummaries []domain.WeeklySummary
	for _, result := range results {
		year, week := result.ID.Year, result.ID.Week
		startDate, endDate := weekToDate(year, week)

		var stressedStudents []domain.StudentInfo
		var overwhelmedStudents []domain.StudentInfo

		for _, student := range result.Students {
			if student.Barometer == "Stretch zone - Overwhelmed" {
				stressedStudents = append(stressedStudents, student)
			} else if student.Barometer == "Panic Zone" {
				overwhelmedStudents = append(overwhelmedStudents, student)
			}
		}

		summary := domain.WeeklySummary{
			WeekStartDate:       startDate.Format("2006-01-02"),
			WeekEndDate:         endDate.Format("2006-01-02"),
			StressedStudents:    stressedStudents,
			OverwhelmedStudents: overwhelmedStudents,
		}
		weeklySummaries = append(weeklySummaries, summary)
	}

	return weeklySummaries, total, nil
}

func weekToDate(year, week int) (time.Time, time.Time) {
	t := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)

	weekday := t.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	t = t.AddDate(0, 0, int(time.Monday-weekday))

	t = t.AddDate(0, 0, (week-1)*7)

	startDate := t
	endDate := t.AddDate(0, 0, 6)

	return startDate, endDate
}
