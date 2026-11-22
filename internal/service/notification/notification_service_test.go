package notification

import (
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type memRepo struct {
	domain.NotificationRepository
	saved *domain.Notification
}

func (m *memRepo) Create(n *domain.Notification) error {
	m.saved = n
	return nil
}

func TestCreateWithRecipients(t *testing.T) {
	r := &memRepo{}
	s := NewService(r)
	a, b := primitive.NewObjectID(), primitive.NewObjectID()
	_, err := s.CreateNotification(map[string]interface{}{
		"title": "Hi", "message": "m", "link": "", "link_text": "", "is_active": true, "priority": "normal",
		"start_date": time.Now(), "end_date": time.Now().Add(time.Hour), "recipient_ids": []primitive.ObjectID{a, b},
	})
	if err != nil || len(r.saved.RecipientIDs) != 2 || r.saved.RecipientIDs[1] != b {
		t.Fatalf("recipients not saved: %+v %v", r.saved, err)
	}
	if _, err := s.CreateNotification(map[string]interface{}{"title": "Hi", "message": "m", "link": "", "link_text": "", "is_active": true, "priority": "", "start_date": time.Now(), "end_date": time.Now()}); err != nil {
		t.Errorf("no recipients still works: %v", err)
	}
}
