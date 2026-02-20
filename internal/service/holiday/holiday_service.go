package holiday

import (
	"context"
	"errors"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	ErrHolidayNotFound = errors.New("holiday not found")
)

type Service struct {
	repo domain.HolidayRepository
	db   *mongo.Database
}

func NewService(repo domain.HolidayRepository, db *mongo.Database) *Service {
	return &Service{repo: repo, db: db}
}

func (s *Service) CreateHoliday(name, startDate, endDate, description, createdBy string) (*domain.Holiday, error) {
	ctx := context.Background()

	holiday := &domain.Holiday{
		Name:        name,
		StartDate:   startDate,
		EndDate:     endDate,
		Description: description,
		CreatedBy:   createdBy,
	}

	if err := s.repo.Insert(ctx, holiday); err != nil {
		return nil, err
	}

	return holiday, nil
}

func (s *Service) GetHolidays() ([]domain.Holiday, error) {
	ctx := context.Background()
	return s.repo.FindAll(ctx)
}

func (s *Service) GetHolidaysInRange(startDate, endDate string) ([]domain.Holiday, error) {
	ctx := context.Background()

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

	collection := s.db.Collection("holidays")
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var holidays []domain.Holiday
	if err := cursor.All(ctx, &holidays); err != nil {
		return nil, err
	}

	return holidays, nil
}

func (s *Service) DeleteHoliday(holidayID string) error {
	ctx := context.Background()

	objID, err := primitive.ObjectIDFromHex(holidayID)
	if err != nil {
		return err
	}

	return s.repo.Delete(ctx, objID)
}

func (s *Service) IsHoliday(date string) (bool, *domain.Holiday, error) {
	ctx := context.Background()

	filter := bson.M{
		"start_date": bson.M{"$lte": date},
		"end_date":   bson.M{"$gte": date},
	}

	collection := s.db.Collection("holidays")
	var holiday domain.Holiday
	err := collection.FindOne(ctx, filter).Decode(&holiday)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil, nil
		}
		return false, nil, err
	}

	return true, &holiday, nil
}

func (s *Service) GetHolidayDatesInRange(startDate, endDate string) (map[string]bool, error) {
	holidays, err := s.GetHolidaysInRange(startDate, endDate)
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
