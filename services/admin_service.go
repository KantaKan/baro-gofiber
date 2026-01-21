package services

import (
	"context"
	"errors"
	"gofiber-baro/config"
	"gofiber-baro/models"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type BarometerData struct {
	Date                             string `json:"date"`
	ComfortZone                      int    `json:"Comfort Zone"`
	PanicZone                        int    `json:"Panic Zone"`
	StretchZoneEnjoyingTheChallenges int    `json:"Stretch Zone - Enjoying the Challenges"`
	StretchZoneOverwhelmed           int    `json:"Stretch Zone - Overwhelmed"`
}

type aggregateResult struct {
	ID struct {
		Date      string `bson:"date"`
		Barometer string `bson:"barometer"`
	} `bson:"_id"`
	Count int `bson:"count"`
}

// mapBarometerToZone converts the raw barometer string (from user reflections)
// into one of the simplified zone IDs expected by the frontend.
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

// GetAllUsers fetches users with optional filters, search, sorting, and pagination.
func GetAllUsers(cohort int, role, email, search, sort string, sortDir, page, limit int) ([]models.User, int, error) {
	if config.DB == nil {
		return nil, 0, errors.New("MongoDB connection is not initialized")
	}

	filter := bson.M{}
	if cohort > 0 {
		filter["cohort_number"] = cohort
	}
	if role != "" {
		filter["role"] = role
	}
	if email != "" {
		filter["email"] = email
	}
	if search != "" {
		filter["$or"] = []bson.M{
			{"first_name": bson.M{"$regex": search, "$options": "i"}},
			{"last_name": bson.M{"$regex": search, "$options": "i"}},
			{"email": bson.M{"$regex": search, "$options": "i"}},
		}
	}

	findOptions := options.Find()
	if limit > 0 {
		findOptions.SetLimit(int64(limit))
	}
	if page > 1 {
		skip := int64((page - 1) * limit)
		findOptions.SetSkip(skip)
	}
	if sort != "" {
		direction := 1
		if sortDir == -1 {
			direction = -1
		}
		findOptions.SetSort(bson.D{{Key: sort, Value: direction}})
	}

	cursor, err := config.DB.Collection("users").Find(context.Background(), filter, findOptions)
	if err != nil {
		log.Printf("Error fetching users: %v", err)
		return nil, 0, errors.New("Error fetching users")
	}
	defer cursor.Close(context.Background())

	var users []models.User
	for cursor.Next(context.Background()) {
		var user models.User
		if err := cursor.Decode(&user); err != nil {
			log.Printf("Error decoding user: %v", err)
			continue
		}
		users = append(users, user)
	}

	if err := cursor.Err(); err != nil {
		log.Printf("Cursor error: %v", err)
		return nil, 0, errors.New("Error processing cursor data")
	}

	total, err := config.DB.Collection("users").CountDocuments(context.Background(), filter)
	if err != nil {
		log.Printf("Error counting users: %v", err)
		return users, 0, nil // Return users with total 0 if count fails
	}

	return users, int(total), nil
}

// GetAllReflections fetches all reflections from all users in the database.
func GetAllReflections() ([]models.Reflection, error) {
	if config.DB == nil {
		return nil, errors.New("MongoDB connection is not initialized")
	}

	// Fetch all users to extract their reflections.
	users, _, err := GetAllUsers(0, "", "", "", "", 0, 0, 0)
	if err != nil {
		log.Printf("Error fetching users: %v", err)
		return nil, errors.New("Error fetching users for reflections")
	}

	var reflections []models.Reflection
	for _, user := range users {
		// Append all reflections from the user.
		reflections = append(reflections, user.Reflections...)
	}

	return reflections, nil
}

// GetUserBarometerData fetches and transforms user reflection data into the 4 zone counts.
func GetUserBarometerData() (map[string]int, error) {
	users, _, err := GetAllUsers(0, "", "", "", "", 0, 0, 0)
	if err != nil {
		return nil, err
	}

	// Initialize counters for each zone.
	zoneCounts := map[string]int{
		"Comfort Zone":                           0,
		"Stretch Zone - Enjoying the Challenges": 0,
		"Stretch Zone - Overwhelmed":             0,
		"Panic Zone":                             0,
	}

	for _, user := range users {
		// Iterate over each reflection for the user.
		for _, reflection := range user.Reflections {
			// Get the barometer zone from the reflection data.
			barometer := reflection.ReflectionData.Barometer

			// Normalize the barometer string to lowercase for case-insensitive comparison.
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

func GetAllReflectionsWithUserInfo(page int, limit int) ([]models.ReflectionWithUser, int, error) {
	offset := (page - 1) * limit

	pipeline := []bson.M{
		{
			"$unwind": "$reflections",
		},
		{
			"$project": bson.M{
				"first_name": "$first_name",
				"last_name":  "$last_name",
				"jsd_number": "$jsd_number",
				"date":       "$reflections.date",
				"reflection": "$reflections.reflection",
			},
		},
		{
			"$sort": bson.M{
				"date": -1,
			},
		},
		{
			"$skip": offset,
		},
		{
			"$limit": limit,
		},
	}

	log.Printf("Executing pipeline: %+v", pipeline)

	// Execute the aggregation pipeline
	cursor, err := config.DB.Collection("users").Aggregate(context.Background(), pipeline, options.Aggregate())
	if err != nil {
		log.Printf("Error executing aggregation: %v", err)
		return nil, 0, errors.New("error fetching reflections with user info")
	}
	defer cursor.Close(context.Background())

	var reflectionsWithUser []models.ReflectionWithUser
	if err := cursor.All(context.Background(), &reflectionsWithUser); err != nil {
		log.Printf("Error decoding reflections: %v", err)
		return nil, 0, errors.New("error processing reflection data")
	}

	// Get the total count of reflections
	countPipeline := []bson.M{
		{
			"$unwind": "$reflections",
		},
		{
			"$count": "total",
		},
	}

	log.Printf("Executing count pipeline: %+v", countPipeline)

	countCursor, err := config.DB.Collection("users").Aggregate(context.Background(), countPipeline, options.Aggregate())
	if err != nil {
		log.Printf("Error executing count aggregation: %v", err)
		return nil, 0, errors.New("error fetching total count of reflections")
	}
	defer countCursor.Close(context.Background())

	var countResult []bson.M
	if err := countCursor.All(context.Background(), &countResult); err != nil {
		log.Printf("Error decoding count result: %v", err)
		return nil, 0, errors.New("error processing count data")
	}

	total := 0
	if len(countResult) > 0 {
		total = int(countResult[0]["total"].(int32))
	}

	log.Printf("Total reflections count: %d", total)

	return reflectionsWithUser, total, nil
}

func GetChartData() ([]map[string]interface{}, error) {
	// Connect to the database
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	if err != nil {
		return nil, err
	}
	defer client.Disconnect(context.TODO())

	// Define the date range for last week (Monday to Friday)
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	lastMonday := now.AddDate(0, 0, -weekday-6)
	lastFriday := lastMonday.AddDate(0, 0, 4)

	// Fetch reflections within the date range
	collection := client.Database("users").Collection("reflections")
	filter := bson.M{
		"date": bson.M{
			"$gte": lastMonday,
			"$lte": lastFriday,
		},
	}

	cursor, err := collection.Find(context.TODO(), filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	// Process the data
	chartData := []map[string]interface{}{
		{"day": "Mon", "DailyReflection": 0},
		{"day": "Tue", "DailyReflection": 0},
		{"day": "Wed", "DailyReflection": 0},
		{"day": "Thu", "DailyReflection": 0},
		{"day": "Fri", "DailyReflection": 0},
	}

	for cursor.Next(context.TODO()) {
		var reflection struct {
			Date time.Time `bson:"date"`
		}
		if err := cursor.Decode(&reflection); err != nil {
			continue
		}

		day := reflection.Date.Weekday()
		switch day {
		case time.Monday:
			chartData[0]["DailyReflection"] = chartData[0]["DailyReflection"].(int) + 1
		case time.Tuesday:
			chartData[1]["DailyReflection"] = chartData[1]["DailyReflection"].(int) + 1
		case time.Wednesday:
			chartData[2]["DailyReflection"] = chartData[2]["DailyReflection"].(int) + 1
		case time.Thursday:
			chartData[3]["DailyReflection"] = chartData[3]["DailyReflection"].(int) + 1
		case time.Friday:
			chartData[4]["DailyReflection"] = chartData[4]["DailyReflection"].(int) + 1
		}
	}

	return chartData, nil
}

func GetAllUsersBarometerData(timeRange string) ([]BarometerData, error) {
	// Calculate date range
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

	// Create pipeline for aggregation
	pipeline := []bson.M{
		{
			"$unwind": "$reflections",
		},
		{
			"$match": bson.M{
				"reflections.date": bson.M{
					"$gte": startDate,
					"$lte": endDate,
				},
			},
		},
		{
			"$group": bson.M{
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
			},
		},
		{
			"$sort": bson.M{
				"_id.date": 1,
			},
		},
	}

	// Log the pipeline
	log.Println("Aggregation pipeline:", pipeline)

	// Execute aggregation
	cursor, err := config.DB.Collection("users").Aggregate(context.Background(), pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	// Process results
	type aggregateResult struct {
		ID struct {
			Date      string `bson:"date"`
			Barometer string `bson:"barometer"`
		} `bson:"_id"`
		Count int `bson:"count"`
	}

	var results []aggregateResult
	if err := cursor.All(context.Background(), &results); err != nil {
		return nil, err
	}

	// Log the raw results
	log.Println("Raw aggregation results:", results)

	// Transform into chart data format
	dataMap := make(map[string]*BarometerData)

	// Initialize all dates in the range
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

	// Fill in the actual data
	for _, result := range results {
		data, exists := dataMap[result.ID.Date]
		if !exists {
			log.Printf("Date %s not found in dataMap", result.ID.Date)
			continue
		}

		log.Printf("Processing result: Date=%s, Barometer=%s, Count=%d", result.ID.Date, result.ID.Barometer, result.Count)

		switch result.ID.Barometer {
		case "Comfort Zone":
			data.ComfortZone = result.Count
		case "Panic Zone":
			data.PanicZone = result.Count
		case "Stretch zone - Enjoying the challenges":
			data.StretchZoneEnjoyingTheChallenges = result.Count
		case "Stretch zone - Overwhelmed":
			data.StretchZoneOverwhelmed = result.Count
		default:
			log.Printf("Unknown barometer value: %s", result.ID.Barometer)
		}
	}

	// Convert map to slice
	var chartData []BarometerData
	for _, data := range dataMap {
		chartData = append(chartData, *data)
	}

	return chartData, nil
}

func GetBarometerData(cursor *mongo.Cursor, startDate, endDate time.Time) (map[string]*BarometerData, error) {
	var results []aggregateResult
	if err := cursor.All(context.Background(), &results); err != nil {
		return nil, err
	}

	// Log the raw results
	log.Println("Raw aggregation results:", results)

	// Transform into chart data format
	dataMap := make(map[string]*BarometerData)

	// Initialize all dates in the range
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

	// Aggregate results into the dataMap
	for _, result := range results {
		dateStr := result.ID.Date
		if data, exists := dataMap[dateStr]; exists {
			switch result.ID.Barometer {
			case "Comfort Zone":
				data.ComfortZone += result.Count
			case "Panic Zone":
				data.PanicZone += result.Count
			case "Stretch Zone - Enjoying the Challenges":
				data.StretchZoneEnjoyingTheChallenges += result.Count
			case "Stretch Zone - Overwhelmed":
				data.StretchZoneOverwhelmed += result.Count
			}
		}
	}

	// Log the final dataMap
	log.Println("Final dataMap:", dataMap)

	return dataMap, nil
}

// GetEmojiZoneTableData fetches all users and processes their reflections
// to create a table of dates (entries) and the corresponding zone for each user.
func GetEmojiZoneTableData() ([]models.EmojiZoneTableData, error) {
	users, _, err := GetAllUsers(0, "", "", "", "", 0, 0, 0)
	if err != nil {
		return nil, err
	}

	var tableData []models.EmojiZoneTableData
	for _, user := range users {
		// Use the user's ZoomName as their identifier.
		data := models.EmojiZoneTableData{
			ZoomName: user.ZoomName,
		}
		// Use a map to ensure that each date only has one entry per user.
		entriesMap := make(map[string]string)
		for _, reflection := range user.Reflections {
			// Format the reflection date as YYYY-MM-DD.
			dateStr := reflection.Date.Format("2006-01-02")
			if _, exists := entriesMap[dateStr]; !exists {
				entriesMap[dateStr] = mapBarometerToZone(reflection.ReflectionData.Barometer)
			}
		}

		// Convert the map to a slice and sort by date.
		var entries []models.EmojiZoneEntry
		for date, zone := range entriesMap {
			entries = append(entries, models.EmojiZoneEntry{
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

func GetWeeklySummary(page, limit int) ([]models.WeeklySummary, int, error) {
	if config.DB == nil {
		return nil, 0, errors.New("MongoDB connection is not initialized")
	}

	collection := config.DB.Collection("users")

	// Common stages for both pipelines
	unwindStage := bson.D{{Key: "$unwind", Value: "$reflections"}}
	matchStage := bson.D{{Key: "$match", Value: bson.D{
		{Key: "reflections.reflection.barometer", Value: bson.D{
			{Key: "$in", Value: bson.A{"Stretch zone - Overwhelmed", "Panic Zone"}},
		}},
	}}}
	groupStage := bson.D{{Key: "$group", Value: bson.D{
		{Key: "_id", Value: bson.D{
			{Key: "year", Value: bson.D{{Key: "$year", Value: "$reflections.date"}}},
			{Key: "week", Value: bson.D{{Key: "$isoWeek", Value: "$reflections.date"}}},
		}},
	}}}
	countStage := bson.D{{Key: "$count", Value: "total"}}

	// Pipeline to count total weeks
	countPipeline := mongo.Pipeline{unwindStage, matchStage, groupStage, countStage}
	countCursor, err := collection.Aggregate(context.Background(), countPipeline)
	if err != nil {
		return nil, 0, err
	}
	var countResult []bson.M
	if err = countCursor.All(context.Background(), &countResult); err != nil {
		return nil, 0, err
	}
	total := 0
	if len(countResult) > 0 {
		total = int(countResult[0]["total"].(int32))
	}
	countCursor.Close(context.Background())

	// Paginated pipeline to get the actual data
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

	cursor, err := collection.Aggregate(context.Background(), paginatedPipeline)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(context.Background())

	var results []struct {
		ID struct {
			Year int `bson:"year"`
			Week int `bson:"week"`
		} `bson:"_id"`
		Students []models.StudentInfo `bson:"students"`
	}

	if err = cursor.All(context.Background(), &results); err != nil {
		return nil, 0, err
	}

	var weeklySummaries []models.WeeklySummary
	for _, result := range results {
		year, week := result.ID.Year, result.ID.Week
		startDate, endDate := weekToDate(year, week)

		var stressedStudents []models.StudentInfo
		var overwhelmedStudents []models.StudentInfo

		for _, student := range result.Students {
			if student.Barometer == "Stretch zone - Overwhelmed" {
				stressedStudents = append(stressedStudents, student)
			} else if student.Barometer == "Panic Zone" {
				overwhelmedStudents = append(overwhelmedStudents, student)
			}
		}

		summary := models.WeeklySummary{
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
	// ISO 8601 weeks start on Monday.
	// The first week of the year is the one that contains the first Thursday.
	// Or equivalently, the one that contains January 4.
	t := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)

	// Adjust to the Monday of that week.
	weekday := t.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	t = t.AddDate(0, 0, int(time.Monday-weekday))

	// Add the number of weeks.
	t = t.AddDate(0, 0, (week-1)*7)

	startDate := t
	endDate := t.AddDate(0, 0, 6)

	return startDate, endDate
}

// AwardBadgeToUser adds a badge to a user's profile, ensuring uniqueness.
func AwardBadgeToUser(userID primitive.ObjectID, badgeType, badgeName, emoji, imageUrl string) error {
	if config.DB == nil {
		return errors.New("MongoDB connection is not initialized")
	}

	collection := config.DB.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Find the user
	var user models.User
	err := collection.FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("user not found")
		}
		log.Printf("Error finding user %s: %v", userID.Hex(), err)
		return errors.New("error finding user")
	}

	// Create new badge object
	newBadge := models.Badge{
		ID:        primitive.NewObjectID(),
		Type:      badgeType,
		Name:      badgeName,
		Emoji:     emoji,
		ImageUrl:  imageUrl,
		AwardedAt: time.Now(),
	}

	// Add the new badge to the badges array
	user.Badges = append(user.Badges, newBadge)

	// Update the user in the database
	_, err = collection.UpdateOne(
		ctx,
		bson.M{"_id": userID},
		bson.M{"$set": bson.M{"badges": user.Badges}},
	)
	if err != nil {
		log.Printf("Error updating user %s with badge %s: %v", userID.Hex(), badgeName, err)
		return errors.New("error awarding badge")
	}

	return nil
}

// UpdateReflectionFeedback updates the admin feedback for a specific reflection of a user.
func UpdateReflectionFeedback(userID, reflectionID primitive.ObjectID, feedbackText string) error {
	if config.DB == nil {
		return errors.New("MongoDB connection is not initialized")
	}

	collection := config.DB.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use an aggregation pipeline to find the user and then update the specific reflection
	// This approach is more robust than fetching the whole user, modifying locally, and then saving.
	// It directly targets the subdocument for update.
	filter := bson.M{
		"_id":             userID,
		"reflections._id": reflectionID,
	}

	update := bson.M{
		"$set": bson.M{
			"reflections.$.admin_feedback": feedbackText, // The positional operator '$' targets the matched element in the array
		},
	}

	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		log.Printf("Error updating reflection feedback for user %s, reflection %s: %v", userID.Hex(), reflectionID.Hex(), err)
		return errors.New("error updating reflection feedback")
	}

	if result.ModifiedCount == 0 {
		return errors.New("user or reflection not found")
	}

	return nil
}
