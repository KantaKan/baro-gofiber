package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gofiber-baro/internal/domain"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// shouldAudit decides which requests are worth a history row: mutating
// requests only, minus a couple of high-volume/no-actor exceptions.
func shouldAudit(method, path string) bool {
	switch method {
	case fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch, fiber.MethodDelete:
	default:
		return false
	}

	if path == "/login" {
		return false
	}
	if strings.HasPrefix(path, "/api/notifications/") && strings.HasSuffix(path, "/read") {
		return false
	}

	return true
}

// AuditMiddleware records every mutating request's actor, route, and outcome
// so admins can review history. It must run before AuthMiddleware sets
// c.Locals("user") (registered as app.Use, ahead of the auth-guarded groups),
// and it never alters the response — logging failures are swallowed.
func AuditMiddleware(repo domain.AuditLogRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		method := c.Method()
		path := c.Path()
		if !shouldAudit(method, path) {
			return c.Next()
		}

		handlerErr := c.Next()

		claims, ok := c.Locals("user").(*Claims)
		if !ok {
			return handlerErr
		}
		actorID, err := primitive.ObjectIDFromHex(claims.UserID)
		if err != nil {
			return handlerErr
		}

		route := path
		if r := c.Route(); r != nil && r.Path != "" {
			route = r.Path
		}

		entry := domain.AuditLog{
			ActorID:   actorID,
			ActorRole: claims.Role,
			Cohort:    claims.Cohort,
			Method:    method,
			Path:      path,
			Route:     route,
			Status:    c.Response().StatusCode(),
			IPAddress: c.IP(),
			CreatedAt: time.Now(),
		}
		entry.Targets, entry.Detail = auditTargets(c.AllParams(), c.Body())

		// ponytail: fire-and-forget insert; add a buffered channel + batch
		// writer if write volume ever shows up in latency or Mongo op counts.
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = repo.Insert(ctx, &entry)
		}()

		return handlerErr
	}
}

// Only these body fields are kept, so free text (leave reasons, notes) and secrets never reach the log.
var auditDetailKeys = []string{"date", "session", "status", "type", "group", "points", "category"}

// auditTargets pulls who a request was for and a few safe facts about it, for the activity log.
func auditTargets(params map[string]string, body []byte) ([]string, map[string]string) {
	var targets []string
	seen := map[string]bool{}
	add := func(id string) {
		if primitive.IsValidObjectID(id) && !seen[id] && len(targets) < 50 {
			seen[id] = true
			targets = append(targets, id)
		}
	}
	for _, k := range []string{"userId", "id"} {
		add(params[k])
	}
	var detail map[string]string
	if len(body) == 0 || len(body) > 64<<10 {
		return targets, detail
	}
	var m map[string]any
	if json.Unmarshal(body, &m) != nil {
		return targets, detail
	}
	if id, ok := m["user_id"].(string); ok {
		add(id)
	}
	if ids, ok := m["user_ids"].([]any); ok {
		for _, v := range ids {
			if id, ok := v.(string); ok {
				add(id)
			}
		}
	}
	for _, k := range auditDetailKeys {
		switch v := m[k].(type) {
		case string:
			if v != "" && len(v) <= 40 {
				if detail == nil {
					detail = map[string]string{}
				}
				detail[k] = v
			}
		case float64:
			if detail == nil {
				detail = map[string]string{}
			}
			detail[k] = fmt.Sprint(v)
		}
	}
	return targets, detail
}
