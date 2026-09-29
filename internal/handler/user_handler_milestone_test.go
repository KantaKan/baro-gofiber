package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type failingMilestoneReconciler struct {
	boxes []domain.TeacherGiftBox
}

func (r failingMilestoneReconciler) Reconcile(context.Context, primitive.ObjectID, time.Time) ([]domain.TeacherGiftBox, error) {
	return r.boxes, errors.New("temporary reward failure")
}

func TestAttachMilestoneRewardsKeepsReflectionSuccessful(t *testing.T) {
	box := domain.TeacherGiftBox{ID: primitive.NewObjectID()}
	handler := &UserHandler{milestones: failingMilestoneReconciler{boxes: []domain.TeacherGiftBox{box}}}
	reflection := &domain.Reflection{ID: primitive.NewObjectID()}

	handler.attachMilestoneRewards(context.Background(), primitive.NewObjectID(), reflection)

	if len(reflection.RewardBoxes) != 1 || reflection.RewardBoxes[0].ID != box.ID {
		t.Fatalf("partial rewards were not preserved: %+v", reflection.RewardBoxes)
	}
	if reflection.RewardWarning == "" {
		t.Fatal("expected non-blocking reward warning")
	}
}
