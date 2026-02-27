package notification

import (
	"gofiber-baro/internal/domain"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Service struct {
	repo domain.NotificationRepository
}

func NewService(repo domain.NotificationRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateNotification(data map[string]interface{}) (*domain.Notification, error) {
	notification := &domain.Notification{
		Title:     data["title"].(string),
		Message:   data["message"].(string),
		Link:      data["link"].(string),
		LinkText:  data["link_text"].(string),
		IsActive:  data["is_active"].(bool),
		Priority:  data["priority"].(string),
		StartDate: data["start_date"].(time.Time),
		EndDate:   data["end_date"].(time.Time),
	}

	if notification.Priority == "" {
		notification.Priority = "normal"
	}

	if err := s.repo.Create(notification); err != nil {
		return nil, err
	}

	return notification, nil
}

func (s *Service) GetAllNotifications() ([]domain.Notification, error) {
	return s.repo.GetAll()
}

func (s *Service) GetActiveNotifications() ([]domain.Notification, error) {
	return s.repo.GetActive()
}

func (s *Service) GetNotificationByID(id string) (*domain.Notification, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	return s.repo.GetByID(objID)
}

func (s *Service) UpdateNotification(id string, updates map[string]interface{}) error {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	return s.repo.Update(objID, updates)
}

func (s *Service) DeleteNotification(id string) error {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	return s.repo.Delete(objID)
}

func (s *Service) MarkAsRead(notificationID, userID string) error {
	notificationObjID, err := primitive.ObjectIDFromHex(notificationID)
	if err != nil {
		return err
	}
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return err
	}
	return s.repo.MarkAsRead(notificationObjID, userObjID)
}

func (s *Service) IsNotificationReadByUser(notification *domain.Notification, userID primitive.ObjectID) bool {
	for _, readUserID := range notification.ReadByUsers {
		if readUserID == userID {
			return true
		}
	}
	return false
}

func (s *Service) GetUnreadNotifications(userID string) ([]domain.Notification, error) {
	notifications, err := s.repo.GetActive()
	if err != nil {
		return nil, err
	}

	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}

	var unreadNotifications []domain.Notification
	for _, notification := range notifications {
		if !s.IsNotificationReadByUser(&notification, userObjID) {
			unreadNotifications = append(unreadNotifications, notification)
		}
	}

	return unreadNotifications, nil
}
