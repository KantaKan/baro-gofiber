package jobs

import (
	"context"
	"log"
	"time"

	"gofiber-baro/internal/repository"

	"go.mongodb.org/mongo-driver/mongo"
)

func RunCohortLockJob(ctx context.Context, db *mongo.Database, interval time.Duration) {
	repo := repository.NewCohortRepository(db)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Println("Cohort lock job started")

	for {
		select {
		case <-ctx.Done():
			log.Println("Cohort lock job stopped")
			return
		case <-ticker.C:
			if err := repo.LockExpired(ctx); err != nil {
				log.Printf("WARNING: cohort lock job error: %v", err)
			}
		}
	}
}
