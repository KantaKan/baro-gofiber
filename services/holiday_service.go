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
)

var (
	ErrHolidayNotFound = errors.New("holiday not found")
)

func CreateHoliday(name, startDate, endDate, description, createdBy string) (*models.Holiday, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	holiday := models.Holiday{
		Name:        name,
		StartDate:   startDate,
		EndDate:     endDate,
		Description: description,
		CreatedAt:   time.Now(),
		CreatedBy:   createdBy,
	}

	result, err := config.HolidaysCollection.InsertOne(ctx, holiday)
	if err != nil {
		return nil, err
	}

	holiday.ID = result.InsertedID.(primitive.ObjectID)
	return &holiday, nil
}

func GetHolidays() ([]models.Holiday, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := config.HolidaysCollection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var holidays []models.Holiday
	if err := cursor.All(ctx, &holidays); err != nil {
		return nil, err
	}

	return holidays, nil
}

func GetHolidaysInRange(startDate, endDate string) ([]models.Holiday, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{
		"$or": []bson.M{
			{"start_date": bson.M{"$gte": startDate, "$lte": endDate}},
			{"end_date": bson.M{"$gte": startDate, "$lte": endDate}},
			{
				"start_date": bson.M{"$lte": endDate},
				"end_date":   bson.M{"$gte": startDate},
			},
		},
	}

	cursor, err := config.HolidaysCollection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var holidays []models.Holiday
	if err := cursor.All(ctx, &holidays); err != nil {
		return nil, err
	}

	return holidays, nil
}

func DeleteHoliday(holidayID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(holidayID)
	if err != nil {
		return err
	}

	result, err := config.HolidaysCollection.DeleteOne(ctx, bson.M{"_id": objID})
	if err != nil {
		return err
	}

	if result.DeletedCount == 0 {
		return ErrHolidayNotFound
	}

	return nil
}

func IsHoliday(date string) (bool, *models.Holiday, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{
		"start_date": bson.M{"$lte": date},
		"end_date":   bson.M{"$gte": date},
	}

	var holiday models.Holiday
	err := config.HolidaysCollection.FindOne(ctx, filter).Decode(&holiday)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil, nil
		}
		return false, nil, err
	}

	return true, &holiday, nil
}

func GetHolidayDatesInRange(startDate, endDate string) (map[string]bool, error) {
	holidays, err := GetHolidaysInRange(startDate, endDate)
	if err != nil {
		return nil, err
	}

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

	return holidayDates, nil
}
