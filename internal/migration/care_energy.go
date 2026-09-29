package migration

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	CareEnergyBalanceField = "care_energy_balance"
	CareEnergyLogField     = "care_energy_log"
	LegacyBalanceField     = "fertilizer_balance"
	LegacyLogField         = "fertilizer_log"
)

type CareEnergySnapshot struct {
	Accounts int64 `bson:"accounts"`
	Balance  int64 `bson:"balance"`
	Entries  int64 `bson:"entries"`
}

func CareEnergyFields(rollback bool) (string, string, string, string) {
	if rollback {
		return LegacyBalanceField, LegacyLogField, CareEnergyBalanceField, CareEnergyLogField
	}
	return CareEnergyBalanceField, CareEnergyLogField, LegacyBalanceField, LegacyLogField
}

func CareEnergyConflictFilter() bson.M {
	return bson.M{"$expr": bson.M{"$or": bson.A{
		bson.M{"$and": bson.A{
			bson.M{"$ne": bson.A{bson.M{"$type": "$" + CareEnergyBalanceField}, "missing"}},
			bson.M{"$ne": bson.A{bson.M{"$type": "$" + LegacyBalanceField}, "missing"}},
			bson.M{"$ne": bson.A{"$" + CareEnergyBalanceField, "$" + LegacyBalanceField}},
		}},
		bson.M{"$and": bson.A{
			bson.M{"$ne": bson.A{bson.M{"$type": "$" + CareEnergyLogField}, "missing"}},
			bson.M{"$ne": bson.A{bson.M{"$type": "$" + LegacyLogField}, "missing"}},
			bson.M{"$ne": bson.A{"$" + CareEnergyLogField, "$" + LegacyLogField}},
		}},
	}}}
}

func CareEnergyMigrationPipeline(rollback bool) mongo.Pipeline {
	targetBalance, targetLog, sourceBalance, sourceLog := CareEnergyFields(rollback)
	return mongo.Pipeline{
		bson.D{{Key: "$set", Value: bson.M{
			targetBalance: bson.M{"$cond": bson.A{
				bson.M{"$ne": bson.A{bson.M{"$type": "$" + targetBalance}, "missing"}},
				"$" + targetBalance,
				bson.M{"$ifNull": bson.A{"$" + sourceBalance, 0}},
			}},
			targetLog: bson.M{"$cond": bson.A{
				bson.M{"$ne": bson.A{bson.M{"$type": "$" + targetLog}, "missing"}},
				"$" + targetLog,
				bson.M{"$ifNull": bson.A{"$" + sourceLog, bson.A{}}},
			}},
		}}},
		bson.D{{Key: "$unset", Value: bson.A{sourceBalance, sourceLog}}},
	}
}

func CareEnergySnapshotPipeline() mongo.Pipeline {
	return mongo.Pipeline{
		bson.D{{Key: "$project", Value: bson.M{
			"balance": bson.M{"$ifNull": bson.A{"$" + CareEnergyBalanceField, bson.M{"$ifNull": bson.A{"$" + LegacyBalanceField, 0}}}},
			"entries": bson.M{"$size": bson.M{"$ifNull": bson.A{"$" + CareEnergyLogField, bson.M{"$ifNull": bson.A{"$" + LegacyLogField, bson.A{}}}}}},
		}}},
		bson.D{{Key: "$group", Value: bson.M{
			"_id":      nil,
			"accounts": bson.M{"$sum": 1},
			"balance":  bson.M{"$sum": "$balance"},
			"entries":  bson.M{"$sum": "$entries"},
		}}},
	}
}

func SameCareEnergySnapshot(left, right CareEnergySnapshot) bool {
	return left == right
}
