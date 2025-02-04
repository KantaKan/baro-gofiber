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
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type BarometerData struct {
	Date                              string `json:"date"`
	ComfortZone                       int    `json:"Comfort Zone"`
	PanicZone                         int    `json:"Panic Zone"`
	StretchZoneEnjoyingTheChallenges  int    `json:"Stretch Zone - Enjoying the Challenges"`
	StretchZoneOverwhelmed            int    `json:"Stretch Zone - Overwhelmed"`
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

// GetAllUsers fetches all users in the database.
func GetAllUsers() ([]models.User, error) {
	if config.DB == nil {
		return nil, errors.New("MongoDB connection is not initialized")
	}

	// Fetch all users from the database.
	cursor, err := config.DB.Collection("users").Find(context.Background(), bson.M{})
	if err != nil {
		log.Printf("Error fetching users: %v", err)
		return nil, errors.New("Error fetching users")
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
		return nil, errors.New("Error processing cursor data")
	}

	return users, nil
}

// GetAllReflections fetches all reflections from all users in the database.
func GetAllReflections() ([]models.Reflection, error) {
	if config.DB == nil {
		return nil, errors.New("MongoDB connection is not initialized")
	}

	// Fetch all users to extract their reflections.
	users, err := GetAllUsers()
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
	users, err := GetAllUsers()
	if err != nil {
		return nil, err
	}

	// Initialize counters for each zone.
	zoneCounts := map[string]int{
		"Comfort Zone":                          0,
		"Stretch Zone - Enjoying the Challenges": 0,
		"Stretch Zone - Overwhelmed":            0,
		"Panic Zone":                            0,
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
                            "date": "$reflections.date",
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
            Date:                              dateStr,
            ComfortZone:                       0,
            PanicZone:                         0,
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
	users, err := GetAllUsers()
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

