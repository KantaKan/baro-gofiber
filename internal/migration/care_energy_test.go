package migration

import (
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func TestCareEnergyMigrationPipelineIsDirectionalAndRepeatable(t *testing.T) {
	conflictFilter, err := bson.MarshalExtJSON(CareEnergyConflictFilter(), false, false)
	if err != nil {
		t.Fatal(err)
	}
	conflictJSON := string(conflictFilter)
	if !strings.Contains(conflictJSON, CareEnergyBalanceField) || !strings.Contains(conflictJSON, LegacyBalanceField) || !strings.Contains(conflictJSON, "$ne") {
		t.Fatalf("conflicting records would not be blocked: %s", conflictJSON)
	}

	forward, err := bson.MarshalExtJSON(bson.M{"pipeline": CareEnergyMigrationPipeline(false)}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	forwardJSON := string(forward)
	if !strings.Contains(forwardJSON, CareEnergyBalanceField) || !strings.Contains(forwardJSON, LegacyBalanceField) || !strings.Contains(forwardJSON, "$unset") {
		t.Fatalf("forward pipeline is incomplete: %s", forwardJSON)
	}
	if !strings.Contains(forwardJSON, "$type") || !strings.Contains(forwardJSON, "$ifNull") {
		t.Fatalf("forward pipeline would not preserve an existing target: %s", forwardJSON)
	}

	rollback, err := bson.MarshalExtJSON(bson.M{"pipeline": CareEnergyMigrationPipeline(true)}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	rollbackJSON := string(rollback)
	if forwardJSON == rollbackJSON || !strings.Contains(rollbackJSON, LegacyLogField) || !strings.Contains(rollbackJSON, CareEnergyLogField) {
		t.Fatalf("rollback pipeline is not the inverse: %s", rollbackJSON)
	}
}

func TestCareEnergySnapshotDetectsLoss(t *testing.T) {
	before := CareEnergySnapshot{Accounts: 40, Balance: 91, Entries: 63}
	if !SameCareEnergySnapshot(before, before) {
		t.Fatal("equal snapshots were rejected")
	}
	if SameCareEnergySnapshot(before, CareEnergySnapshot{Accounts: 40, Balance: 90, Entries: 63}) {
		t.Fatal("lost balance was not detected")
	}
	if SameCareEnergySnapshot(before, CareEnergySnapshot{Accounts: 40, Balance: 91, Entries: 62}) {
		t.Fatal("lost history was not detected")
	}
}
