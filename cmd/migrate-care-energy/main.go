package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"gofiber-baro/internal/migration"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	apply := flag.Bool("apply", false, "apply the migration; default is dry run")
	rollback := flag.Bool("rollback", false, "restore the previous field names")
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
	conflicts, err := collection.CountDocuments(ctx, migration.CareEnergyConflictFilter())
	if err != nil {
		log.Fatal(err)
	}
	if conflicts > 0 {
		log.Fatalf("aborted: %d users have conflicting old and new Care Energy values", conflicts)
	}

	before, err := snapshot(ctx, collection)
	if err != nil {
		log.Fatal(err)
	}
	mode := "forward"
	if *rollback {
		mode = "rollback"
	}
	if !*apply {
		fmt.Printf("mode=dry-run direction=%s accounts=%d balance=%d entries=%d conflicts=%d\n", mode, before.Accounts, before.Balance, before.Entries, conflicts)
		return
	}

	result, err := collection.UpdateMany(ctx, bson.M{}, migration.CareEnergyMigrationPipeline(*rollback))
	if err != nil {
		log.Fatal(err)
	}
	after, err := snapshot(ctx, collection)
	if err != nil {
		log.Fatal(err)
	}
	if !migration.SameCareEnergySnapshot(before, after) {
		log.Fatalf("verification failed: before=%+v after=%+v", before, after)
	}
	fmt.Printf("mode=apply direction=%s matched=%d modified=%d accounts=%d balance=%d entries=%d verified=true\n", mode, result.MatchedCount, result.ModifiedCount, after.Accounts, after.Balance, after.Entries)
}

func snapshot(ctx context.Context, collection *mongo.Collection) (migration.CareEnergySnapshot, error) {
	cursor, err := collection.Aggregate(ctx, migration.CareEnergySnapshotPipeline())
	if err != nil {
		return migration.CareEnergySnapshot{}, err
	}
	defer cursor.Close(ctx)
	if !cursor.Next(ctx) {
		return migration.CareEnergySnapshot{}, cursor.Err()
	}
	var result migration.CareEnergySnapshot
	if err := cursor.Decode(&result); err != nil {
		return migration.CareEnergySnapshot{}, err
	}
	return result, nil
}
