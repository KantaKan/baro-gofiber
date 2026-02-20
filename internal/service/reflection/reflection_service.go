package reflection

import (
	"context"
	"sort"
	"strings"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Service struct {
	db *mongo.Database
}

func NewService(db *mongo.Database) *Service {
	return &Service{db: db}
}

func (s *Service) GetAllReflections() ([]domain.Reflection, error) {
	ctx := context.Background()

	pipeline := []bson.M{
		{"$unwind": "$reflections"},
		{"$project": bson.M{
			"_id":            "$reflections._id",
			"day":            "$reflections.day",
			"user_id":        "$reflections.user_id",
			"date":           "$reflections.date",
			"created_at":     "$reflections.createdAt",
			"reflection":     "$reflections.reflection",
			"admin_feedback": "$reflections.admin_feedback",
		}},
	}

	cursor, err := s.db.Collection("users").Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var reflections []domain.Reflection
	if err := cursor.All(ctx, &reflections); err != nil {
		return nil, err
	}

	return reflections, nil
}

func (s *Service) GetAllReflectionsWithUserInfo(page int, limit int) ([]map[string]interface{}, int, error) {
	ctx := context.Background()
	offset := (page - 1) * limit

	pipeline := []bson.M{
		{"$unwind": "$reflections"},
		{"$project": bson.M{
			"_id":       "$reflections._id",
			"user_id":   bson.M{"$toString": "$_id"},
			"FirstName": "$first_name",
			"LastName":  "$last_name",
			"JsdNumber": "$jsd_number",
			"Date":      "$reflections.date",
			"Reflection": bson.M{
				"Barometer": "$reflections.reflection.barometer",
				"TechSessions": bson.M{
					"SessionName": "$reflections.reflection.tech_sessions.session_name",
					"Happy":       "$reflections.reflection.tech_sessions.happy",
					"Improve":     "$reflections.reflection.tech_sessions.improve",
				},
				"NonTechSessions": bson.M{
					"SessionName": "$reflections.reflection.non_tech_sessions.session_name",
					"Happy":       "$reflections.reflection.non_tech_sessions.happy",
					"Improve":     "$reflections.reflection.non_tech_sessions.improve",
				},
			},
		}},
		{"$sort": bson.M{"Date": -1}},
		{"$skip": offset},
		{"$limit": limit},
	}

	cursor, err := s.db.Collection("users").Aggregate(ctx, pipeline, options.Aggregate())
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var reflections []map[string]interface{}
	if err := cursor.All(ctx, &reflections); err != nil {
		return nil, 0, err
	}

	countPipeline := []bson.M{
		{"$unwind": "$reflections"},
		{"$count": "total"},
	}

	countCursor, err := s.db.Collection("users").Aggregate(ctx, countPipeline, options.Aggregate())
	if err != nil {
		return nil, 0, err
	}
	defer countCursor.Close(ctx)

	var countResult []bson.M
	if err := countCursor.All(ctx, &countResult); err != nil {
		return nil, 0, err
	}

	total := 0
	if len(countResult) > 0 {
		total = int(countResult[0]["total"].(int32))
	}

	return reflections, total, nil
}

func (s *Service) GetEmojiZoneTableData(users []domain.User) ([]domain.EmojiZoneTableData, error) {
	var tableData []domain.EmojiZoneTableData

	for _, user := range users {
		data := domain.EmojiZoneTableData{
			ZoomName: user.ZoomName,
		}

		entriesMap := make(map[string]string)
		for _, reflection := range user.Reflections {
			dateStr := reflection.Date.Format("2006-01-02")
			if _, exists := entriesMap[dateStr]; !exists {
				entriesMap[dateStr] = mapBarometerToZone(reflection.ReflectionData.Barometer)
			}
		}

		var entries []domain.EmojiZoneEntry
		for date, zone := range entriesMap {
			entries = append(entries, domain.EmojiZoneEntry{
				Date: date,
				Zone: zone,
			})
		}

		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Date < entries[j].Date
		})

		data.Entries = entries
		tableData = append(tableData, data)
	}

	return tableData, nil
}

func mapBarometerToZone(barometer string) string {
	switch strings.ToLower(barometer) {
	case "comfort zone":
		return "comfort"
	case "stretch zone - enjoying the challenges":
		return "stretch-enjoying"
	case "stretch zone - overwhelmed":
		return "stretch-overwhelmed"
	case "panic zone":
		return "panic"
	default:
		return "no-data"
	}
}
