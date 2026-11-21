package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"gofiber-baro/internal/domain"
	userservice "gofiber-baro/internal/service/user"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	apply := flag.Bool("apply", false, "apply the idempotent backfill; default is dry run")
	flag.Parse()

	uri := os.Getenv("MONGO_URI")
	databaseName := os.Getenv("DATABASE_NAME")
	if uri == "" || databaseName == "" {
		log.Fatal("MONGO_URI and DATABASE_NAME are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(context.Background())
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatal(err)
	}

	collection := client.Database(databaseName).Collection("users")
	cursor, err := collection.Find(ctx, bson.M{"$or": bson.A{
		bson.M{"selected_palette": bson.M{"$exists": true, "$nin": bson.A{nil, ""}}},
		bson.M{"selected_pot": bson.M{"$exists": true, "$nin": bson.A{nil, ""}}},
	}})
	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(ctx)

	var scanned, planned, ownedUpdates, equipUpdates int
	for cursor.Next(ctx) {
		var learner domain.User
		if err := cursor.Decode(&learner); err != nil {
			log.Fatal(err)
		}
		scanned++
		plan := userservice.PlanLegacyCosmeticBackfill(&learner)
		if len(plan.OwnedIDs) == 0 && len(plan.Equip) == 0 {
			continue
		}
		planned++
		if !*apply {
			continue
		}
		if len(plan.OwnedIDs) > 0 {
			result, err := collection.UpdateOne(ctx, bson.M{"_id": learner.ID}, bson.M{
				"$addToSet": bson.M{"owned_cosmetic_ids": bson.M{"$each": plan.OwnedIDs}},
			})
			if err != nil {
				log.Fatal(err)
			}
			ownedUpdates += int(result.ModifiedCount)
		}
		for slot, cosmeticID := range plan.Equip {
			field := "equipped_cosmetics." + slot
			result, err := collection.UpdateOne(ctx, bson.M{
				"_id": learner.ID,
				field: bson.M{"$in": bson.A{nil, ""}},
			}, bson.M{"$set": bson.M{field: cosmeticID}})
			if err != nil {
				log.Fatal(err)
			}
			equipUpdates += int(result.ModifiedCount)
		}
	}
	if err := cursor.Err(); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("mode=%s scanned=%d planned=%d ownership_updates=%d equip_updates=%d\n", map[bool]string{true: "apply", false: "dry-run"}[*apply], scanned, planned, ownedUpdates, equipUpdates)
}
