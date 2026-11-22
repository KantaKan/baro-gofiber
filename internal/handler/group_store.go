package handler

import (
	"context"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GroupStore joins user lookup and the group update.
type GroupStore struct {
	Lookup interface {
		GetUserByID(id string) (*domain.User, error)
	}
	Setter interface {
		SetGenmateGroup(ctx context.Context, id primitive.ObjectID, group string) error
	}
}

func (g GroupStore) GetUserByID(id string) (*domain.User, error) { return g.Lookup.GetUserByID(id) }
func (g GroupStore) SetGenmateGroup(ctx context.Context, id primitive.ObjectID, group string) error {
	return g.Setter.SetGenmateGroup(ctx, id, group)
}
