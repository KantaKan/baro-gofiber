package repository

import (
	"testing"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestRecordedGiftBoxDrawKeepsItsSourcePool(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		pool   string
	}{
		{name: "teacher", source: "", pool: "teacher-box"},
		{name: "reflection", source: "reflection-milestone", pool: "reflection"},
		{name: "achievement", source: "achievement", pool: "achievement"},
	} {
		t.Run(test.name, func(t *testing.T) {
			box := domain.TeacherGiftBox{
				ID: primitive.NewObjectID(), Source: test.source,
				Reward: &domain.CosmeticCatalogItem{ID: "palette:ocean"},
			}
			if result := drawResult(primitive.NewObjectID(), box); result.Pool != test.pool {
				t.Fatalf("recorded draw pool = %q, want %q", result.Pool, test.pool)
			}
		})
	}
}
