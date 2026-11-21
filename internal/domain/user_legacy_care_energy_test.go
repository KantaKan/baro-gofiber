package domain

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func TestUserDecodesLegacyFertilizerAsCareEnergy(t *testing.T) {
	createdAt := time.Date(2026, time.September, 14, 9, 0, 0, 0, time.UTC)
	document, err := bson.Marshal(bson.M{
		"first_name":         "Legacy learner",
		"fertilizer_balance": 4,
		"fertilizer_log": bson.A{bson.M{
			"kind":        "protect",
			"amount":      1,
			"relatedDate": "2026-09-12",
			"createdAt":   createdAt,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	var user User
	if err := bson.Unmarshal(document, &user); err != nil {
		t.Fatal(err)
	}

	if user.CareEnergyBalance != 4 {
		t.Fatalf("legacy fertilizer balance disappeared: got %d", user.CareEnergyBalance)
	}
	if len(user.CareEnergyLog) != 1 || user.CareEnergyLog[0].Kind != "protect" || user.CareEnergyLog[0].RelatedDate != "2026-09-12" {
		t.Fatalf("legacy streak protection history disappeared: %+v", user.CareEnergyLog)
	}
}

func TestUserPrefersCanonicalCareEnergyOverLegacyFertilizer(t *testing.T) {
	canonicalDate := "2026-09-15"
	document, err := bson.Marshal(bson.M{
		"care_energy_balance": 0,
		"care_energy_log": bson.A{bson.M{
			"kind":        "protect",
			"amount":      1,
			"relatedDate": canonicalDate,
		}},
		"fertilizer_balance": 4,
		"fertilizer_log": bson.A{bson.M{
			"kind":        "protect",
			"amount":      1,
			"relatedDate": "2026-09-12",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	var user User
	if err := bson.Unmarshal(document, &user); err != nil {
		t.Fatal(err)
	}

	if user.CareEnergyBalance != 0 {
		t.Fatalf("legacy balance replaced canonical balance: got %d", user.CareEnergyBalance)
	}
	if len(user.CareEnergyLog) != 1 || user.CareEnergyLog[0].RelatedDate != canonicalDate {
		t.Fatalf("legacy log replaced canonical log: %+v", user.CareEnergyLog)
	}
}
