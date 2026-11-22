package handler

import (
	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/notification"
	"gofiber-baro/internal/service/staff"
	"gofiber-baro/pkg/utils"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type NotificationHandler struct {
	notificationService *notification.Service
	users               staffLookup // for the TA own-groups rule; nil = no check
	slackPoster         interface{ AnnounceAsync(title, message string) }
}

// WithSlack lets an announcement also be posted to Slack ("post_to_slack").
func (h *NotificationHandler) WithSlack(p interface{ AnnounceAsync(title, message string) }) *NotificationHandler {
	h.slackPoster = p
	return h
}

// WithUsers enables the TA own-groups check on new announcements.
func (h *NotificationHandler) WithUsers(u staffLookup) *NotificationHandler {
	h.users = u
	return h
}

func NewNotificationHandler(notificationService *notification.Service) *NotificationHandler {
	return &NotificationHandler{notificationService: notificationService}
}

type CreateNotificationRequest struct {
	Title     string `json:"title"`
	Message   string `json:"message"`
	Link      string `json:"link"`
	LinkText  string `json:"link_text"`
	IsActive  bool   `json:"is_active"`
	Priority  string `json:"priority"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	// RecipientIDs limits the announcement to these learners (empty = everyone).
	RecipientIDs []string `json:"recipient_ids"`
	// PostToSlack also posts the title and message to the announcements channel.
	PostToSlack bool `json:"post_to_slack"`
}

func (h *NotificationHandler) CreateNotification(c *fiber.Ctx) error {
	var body CreateNotificationRequest
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if body.Title == "" || body.Message == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Title and message are required")
	}

	if body.Priority == "" {
		body.Priority = "normal"
	}

	startDate, err := time.Parse(time.RFC3339, body.StartDate)
	if err != nil {
		startDate, err = time.Parse("2006-01-02T15:04:05Z", body.StartDate)
		if err != nil {
			return utils.SendError(c, fiber.StatusBadRequest, "Invalid start date format")
		}
	}

	endDate, err := time.Parse(time.RFC3339, body.EndDate)
	if err != nil {
		endDate, err = time.Parse("2006-01-02T15:04:05Z", body.EndDate)
		if err != nil {
			return utils.SendError(c, fiber.StatusBadRequest, "Invalid end date format")
		}
	}

	data := map[string]interface{}{
		"title":      body.Title,
		"message":    body.Message,
		"link":       body.Link,
		"link_text":  body.LinkText,
		"is_active":  body.IsActive,
		"priority":   body.Priority,
		"start_date": startDate,
		"end_date":   endDate,
	}
	if len(body.RecipientIDs) > 0 {
		ids := make([]primitive.ObjectID, 0, len(body.RecipientIDs))
		for _, s := range body.RecipientIDs {
			id, err := primitive.ObjectIDFromHex(s)
			if err != nil {
				return utils.SendError(c, fiber.StatusBadRequest, "Invalid recipient ID")
			}
			ids = append(ids, id)
		}
		data["recipient_ids"] = ids
	}
	if s, ok := StaffFrom(c); ok && s.Role == staff.TA {
		var recips []*domain.User
		for _, id := range body.RecipientIDs {
			var u *domain.User
			if h.users != nil {
				u, _ = h.users.GetUserByID(id)
			}
			recips = append(recips, u)
		}
		if !taMayAnnounce(s, recips) {
			return utils.SendError(c, fiber.StatusForbidden, "TAs can only send to learners in their own groups")
		}
	}

	notif, err := h.notificationService.CreateNotification(data)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error creating notification")
	}

	if body.PostToSlack && h.slackPoster != nil {
		h.slackPoster.AnnounceAsync(body.Title, body.Message)
	}

	return utils.SendResponse(c, fiber.StatusOK, "Notification created", notif)
}

func (h *NotificationHandler) GetAllNotifications(c *fiber.Ctx) error {
	notifications, err := h.notificationService.GetAllNotifications()
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching notifications: "+err.Error())
	}

	if notifications == nil {
		notifications = []domain.Notification{}
	}

	return utils.SendResponse(c, fiber.StatusOK, "Notifications retrieved", notifications)
}

func (h *NotificationHandler) GetActiveNotifications(c *fiber.Ctx) error {
	userID := c.Locals("userID")
	userIDStr, _ := userID.(string)

	notifications, err := h.notificationService.GetActiveNotificationsForUser(userIDStr)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching notifications: "+err.Error())
	}

	if notifications == nil {
		notifications = []domain.Notification{}
	}

	if userIDStr != "" {
		var unreadNotifications []map[string]interface{}
		for _, n := range notifications {
			notifMap := map[string]interface{}{
				"id":         n.ID.Hex(),
				"title":      n.Title,
				"message":    n.Message,
				"link":       n.Link,
				"link_text":  n.LinkText,
				"is_active":  n.IsActive,
				"priority":   n.Priority,
				"start_date": n.StartDate,
				"end_date":   n.EndDate,
				"created_at": n.CreatedAt,
				"is_read": h.notificationService.IsNotificationReadByUser(&n, func() primitive.ObjectID {
					id, _ := primitive.ObjectIDFromHex(userIDStr)
					return id
				}()),
			}
			unreadNotifications = append(unreadNotifications, notifMap)
		}
		return utils.SendResponse(c, fiber.StatusOK, "Active notifications retrieved", unreadNotifications)
	}

	return utils.SendResponse(c, fiber.StatusOK, "Active notifications retrieved", notifications)
}

func (h *NotificationHandler) UpdateNotification(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Notification ID is required")
	}

	var body map[string]interface{}
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if err := h.notificationService.UpdateNotification(id, body); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error updating notification")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Notification updated", nil)
}

func (h *NotificationHandler) DeleteNotification(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Notification ID is required")
	}

	if err := h.notificationService.DeleteNotification(id); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error deleting notification")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Notification deleted", nil)
}

func (h *NotificationHandler) MarkAsRead(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Notification ID is required")
	}

	userID := c.Locals("userID")
	userIDStr, ok := userID.(string)
	if !ok || userIDStr == "" {
		return utils.SendError(c, fiber.StatusUnauthorized, "User not authenticated")
	}

	if err := h.notificationService.MarkAsRead(id, userIDStr); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error marking notification as read")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Notification marked as read", nil)
}
