package middleware

import (
	"context"
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
