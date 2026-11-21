package handler

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type HistoryHandler struct {
	auditRepo domain.AuditLogRepository
	userRepo  domain.UserRepository
}

func NewHistoryHandler(auditRepo domain.AuditLogRepository, userRepo domain.UserRepository) *HistoryHandler {
	return &HistoryHandler{auditRepo: auditRepo, userRepo: userRepo}
}

func (h *HistoryHandler) GetHistory(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	if page < 1 {
		page = 1
	}
	limit := c.QueryInt("limit", 50)
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	filter := bson.M{}
	if role := c.Query("role"); role != "" {
		filter["actor_role"] = role
	}
	if cohortStr := c.Query("cohort"); cohortStr != "" {
		if cohort, err := strconv.Atoi(cohortStr); err == nil {
			filter["cohort"] = cohort
		}
	}
	if userID := c.Query("user_id"); userID != "" {
		if oid, err := primitive.ObjectIDFromHex(userID); err == nil {
			filter["actor_id"] = oid
		}
	}
	if method := c.Query("method"); method != "" {
		filter["method"] = strings.ToUpper(method)
	}
	if q := c.Query("q"); q != "" {
		filter["path"] = bson.M{"$regex": regexp.QuoteMeta(q), "$options": "i"}
	}

	ctx := context.Background()
	logs, total, err := h.auditRepo.FindPage(ctx, filter, page, limit)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching history")
	}

	names := make(map[primitive.ObjectID]string)
	for i := range logs {
		id := logs[i].ActorID
		name, ok := names[id]
		if !ok {
			name = "Unknown"
			if u, err := h.userRepo.FindByID(ctx, id); err == nil && u != nil {
				name = strings.TrimSpace(u.FirstName + " " + u.LastName)
			}
			names[id] = name
		}
		logs[i].ActorName = name
	}

	return utils.SendResponse(c, fiber.StatusOK, "History retrieved", fiber.Map{
		"logs":  logs,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}
