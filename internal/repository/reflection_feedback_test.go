package repository

import (
	"reflect"
	"testing"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
)

func TestReflectionFeedbackUpdate(t *testing.T) {
	at := time.Date(2026, 11, 23, 18, 0, 0, 0, time.UTC)
	author := domain.FeedbackAuthor{ID: "abc", Name: "00_Kan(Kan/กัน)", At: at}

	got := reflectionFeedbackUpdate("Nice work", author)
	want := bson.M{
		"$set": bson.M{
			"reflections.$.admin_feedback":      "Nice work",
			"reflections.$.admin_feedback_by":   "abc",
			"reflections.$.admin_feedback_name": "00_Kan(Kan/กัน)",
			"reflections.$.admin_feedback_at":   at,
		},
		"$unset": bson.M{"reflections.$.admin_feedback_avatar": ""},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("reply: got %v, want %v", got, want)
	}

	cleared := reflectionFeedbackUpdate("  ", author)
	wantCleared := bson.M{
		"$set":   bson.M{"reflections.$.admin_feedback": ""},
		"$unset": bson.M{"reflections.$.admin_feedback_by": "", "reflections.$.admin_feedback_name": "", "reflections.$.admin_feedback_at": "", "reflections.$.admin_feedback_avatar": ""},
	}
	if !reflect.DeepEqual(cleared, wantCleared) {
		t.Errorf("clearing: got %v, want %v", cleared, wantCleared)
	}

	unknown := reflectionFeedbackUpdate("Nice work", domain.FeedbackAuthor{At: at})
	wantUnknown := bson.M{
		"$set":   bson.M{"reflections.$.admin_feedback": "Nice work", "reflections.$.admin_feedback_at": at},
		"$unset": bson.M{"reflections.$.admin_feedback_by": "", "reflections.$.admin_feedback_name": "", "reflections.$.admin_feedback_avatar": ""},
	}
	if !reflect.DeepEqual(unknown, wantUnknown) {
		t.Errorf("unknown replier: got %v, want %v", unknown, wantUnknown)
	}

	av := &domain.Avatar{Style: "thumbs", Seed: "k"}
	withAvatar := reflectionFeedbackUpdate("Nice", domain.FeedbackAuthor{ID: "abc", Name: "Kan", At: at, Avatar: av})
	if withAvatar["$set"].(bson.M)["reflections.$.admin_feedback_avatar"] != av {
		t.Errorf("reply should keep the replier's avatar: %v", withAvatar)
	}
	noAvatar := reflectionFeedbackUpdate("Nice", domain.FeedbackAuthor{ID: "abc", Name: "Kan", At: at})
	if _, ok := noAvatar["$unset"].(bson.M)["reflections.$.admin_feedback_avatar"]; !ok {
		t.Errorf("a replier without an avatar should clear an old one: %v", noAvatar)
	}
	for _, u := range []bson.M{cleared, unknown} {
		if _, ok := u["$unset"].(bson.M)["reflections.$.admin_feedback_avatar"]; !ok {
			t.Errorf("clearing / unknown replier should clear the avatar: %v", u)
		}
	}
}
