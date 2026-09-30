package integration_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/migration"
	"gofiber-baro/internal/repository"
	"gofiber-baro/internal/service/character"
	"gofiber-baro/internal/service/giftbox"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestCharacterReleaseAgainstMongoDB(t *testing.T) {
	if os.Getenv("BARO_RUN_PERSISTENCE_INTEGRATION") != "1" {
		t.Skip("set BARO_RUN_PERSISTENCE_INTEGRATION=1 to run the isolated MongoDB release check")
	}
	_ = godotenv.Load("../../.env")
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Fatal("MONGO_URI is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetServerSelectionTimeout(15*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.Background())
	if err := client.Ping(ctx, nil); err != nil {
		t.Fatal(err)
	}

	databaseName := "baro_rc_" + primitive.NewObjectID().Hex()
	database := client.Database(databaseName)
	defer func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer dropCancel()
		if err := database.Drop(dropCtx); err != nil {
			t.Errorf("drop isolated database %s: %v", databaseName, err)
		}
	}()

	users := database.Collection("users")
	ownerIDs := make([]primitive.ObjectID, 45)
	documents := make([]interface{}, 45)
	for index := range ownerIDs {
		ownerIDs[index] = primitive.NewObjectID()
		documents[index] = bson.M{
			"_id": ownerIDs[index], "email": fmt.Sprintf("release-%02d@example.test", index),
			"role": "learner", "cohort_number": 99, "deleted": false,
			migration.LegacyBalanceField: index % 6,
			migration.LegacyLogField:     bson.A{bson.M{"kind": "grant", "amount": index % 6}},
		}
	}
	if _, err := users.InsertMany(ctx, documents); err != nil {
		t.Fatal(err)
	}

	before := careEnergySnapshot(t, ctx, users)
	if _, err := users.UpdateMany(ctx, bson.M{}, migration.CareEnergyMigrationPipeline(false)); err != nil {
		t.Fatal(err)
	}
	after := careEnergySnapshot(t, ctx, users)
	if !migration.SameCareEnergySnapshot(before, after) {
		t.Fatalf("care energy migration changed totals: before=%+v after=%+v", before, after)
	}
	if count, err := users.CountDocuments(ctx, bson.M{migration.LegacyBalanceField: bson.M{"$exists": true}}); err != nil || count != 0 {
		t.Fatalf("legacy balance remains after migration: count=%d err=%v", count, err)
	}
	if _, err := users.UpdateMany(ctx, bson.M{}, migration.CareEnergyMigrationPipeline(true)); err != nil {
		t.Fatal(err)
	}
	if rolledBack := careEnergySnapshot(t, ctx, users); !migration.SameCareEnergySnapshot(before, rolledBack) {
		t.Fatalf("care energy rollback changed totals: before=%+v rollback=%+v", before, rolledBack)
	}
	if _, err := users.UpdateMany(ctx, bson.M{}, migration.CareEnergyMigrationPipeline(false)); err != nil {
		t.Fatal(err)
	}

	characterRepository := repository.NewBaroCharacterRepository(database)
	if err := characterRepository.EnsureIndexes(ctx); err != nil {
		t.Fatal(err)
	}
	characterService := character.NewService(characterRepository, character.SecurePicker{})
	type revealResult struct {
		ownerID primitive.ObjectID
		item    *domain.BaroCharacter
		err     error
	}
	reveals := make(chan revealResult, len(ownerIDs)*2)
	var revealGroup sync.WaitGroup
	for _, ownerID := range ownerIDs {
		for range 2 {
			revealGroup.Add(1)
			go func(id primitive.ObjectID) {
				defer revealGroup.Done()
				item, revealErr := characterService.RevealStarter(ctx, id.Hex())
				reveals <- revealResult{ownerID: id, item: item, err: revealErr}
			}(ownerID)
		}
	}
	revealGroup.Wait()
	close(reveals)
	seenByOwner := map[primitive.ObjectID]primitive.ObjectID{}
	for result := range reveals {
		if result.err != nil {
			t.Fatalf("reveal %s: %v", result.ownerID.Hex(), result.err)
		}
		if result.item == nil || result.item.OwnerID != result.ownerID {
			t.Fatalf("reveal returned the wrong owner: owner=%s item=%+v", result.ownerID.Hex(), result.item)
		}
		if prior, exists := seenByOwner[result.ownerID]; exists && prior != result.item.ID {
			t.Fatalf("starter reveal was not idempotent for %s", result.ownerID.Hex())
		}
		seenByOwner[result.ownerID] = result.item.ID
	}
	characterCount, err := database.Collection("baro_characters").CountDocuments(ctx, bson.M{})
	if err != nil || characterCount != int64(len(ownerIDs)) || len(seenByOwner) != len(ownerIDs) {
		t.Fatalf("unexpected starter population: documents=%d owners=%d err=%v", characterCount, len(seenByOwner), err)
	}
	distinctFingerprints, err := database.Collection("baro_characters").Distinct(ctx, "fingerprint", bson.M{})
	if err != nil || len(distinctFingerprints) != len(ownerIDs) {
		t.Fatalf("fingerprints are not unique: count=%d err=%v", len(distinctFingerprints), err)
	}

	careOwner := ownerIDs[0]
	if _, err := users.UpdateOne(ctx, bson.M{"_id": careOwner}, bson.M{"$set": bson.M{
		migration.CareEnergyBalanceField: 5, "character_care_count": 0, "growth_points": 77,
	}}); err != nil {
		t.Fatal(err)
	}
	userRepository := repository.NewUserRepository(database)
	careResults := make(chan error, 10)
	var careGroup sync.WaitGroup
	for range 10 {
		careGroup.Add(1)
		go func() {
			defer careGroup.Done()
			careResults <- userRepository.UseCareEnergyCharacter(ctx, careOwner, "release-check")
		}()
	}
	careGroup.Wait()
	close(careResults)
	successes := 0
	insufficient := 0
	for careErr := range careResults {
		switch {
		case careErr == nil:
			successes++
		case errors.Is(careErr, domain.ErrInsufficientCareEnergy):
			insufficient++
		default:
			t.Fatalf("unexpected Care Energy error: %v", careErr)
		}
	}
	var persisted struct {
		Balance     int `bson:"care_energy_balance"`
		CareCount   int `bson:"character_care_count"`
		GrowthPoint int `bson:"growth_points"`
	}
	if err := users.FindOne(ctx, bson.M{"_id": careOwner}).Decode(&persisted); err != nil {
		t.Fatal(err)
	}
	if successes != 5 || insufficient != 5 || persisted.Balance != 0 || persisted.CareCount != 5 || persisted.GrowthPoint != 77 {
		t.Fatalf("atomic Care Energy spend failed: successes=%d insufficient=%d persisted=%+v", successes, insufficient, persisted)
	}

	boxID := primitive.NewObjectID()
	box := domain.TeacherGiftBox{
		ID: boxID, UserID: ownerIDs[1], MinimumRarity: "Rare", Message: "release check",
		GrantedBy: ownerIDs[4], Status: "unopened", CreatedAt: time.Now().UTC(), RewardPool: "character-box",
	}
	if _, err := users.UpdateOne(ctx, bson.M{"_id": ownerIDs[1]}, bson.M{"$set": bson.M{"first_name": "Sender"}, "$push": bson.M{"gift_boxes": box}}); err != nil {
		t.Fatal(err)
	}
	for index, name := range []string{"First recipient", "Second recipient"} {
		if _, err := users.UpdateOne(ctx, bson.M{"_id": ownerIDs[index+2]}, bson.M{"$set": bson.M{"first_name": name}}); err != nil {
			t.Fatal(err)
		}
	}
	giftRepository := repository.NewGiftBoxRepository(database)
	transferErrors := make(chan error, 2)
	var transferGroup sync.WaitGroup
	for _, recipientID := range ownerIDs[2:4] {
		transferGroup.Add(1)
		go func(id primitive.ObjectID) {
			defer transferGroup.Done()
			_, transferErr := giftRepository.Transfer(ctx, boxID, ownerIDs[1], id)
			transferErrors <- transferErr
		}(recipientID)
	}
	transferGroup.Wait()
	close(transferErrors)
	transferred := 0
	rejected := 0
	for transferErr := range transferErrors {
		switch {
		case transferErr == nil:
			transferred++
		case errors.Is(transferErr, giftbox.ErrBoxNotFound):
			rejected++
		default:
			t.Fatalf("unexpected gift transfer error: %v", transferErr)
		}
	}
	ownersWithBox, err := users.CountDocuments(ctx, bson.M{"gift_boxes._id": boxID})
	if err != nil || transferred != 1 || rejected != 1 || ownersWithBox != 1 {
		t.Fatalf("gift transfer race was not atomic: transferred=%d rejected=%d owners=%d err=%v", transferred, rejected, ownersWithBox, err)
	}
	var currentOwner domain.User
	if err := users.FindOne(ctx, bson.M{"gift_boxes._id": boxID}).Decode(&currentOwner); err != nil {
		t.Fatal(err)
	}
	var persistedBox *domain.TeacherGiftBox
	for index := range currentOwner.GiftBoxes {
		if currentOwner.GiftBoxes[index].ID == boxID {
			persistedBox = &currentOwner.GiftBoxes[index]
			break
		}
	}
	if persistedBox == nil || persistedBox.UserID != currentOwner.ID || len(persistedBox.TransferHistory) != 1 {
		t.Fatalf("gift transfer history or owner is invalid: owner=%s box=%+v", currentOwner.ID.Hex(), persistedBox)
	}

	eggOwner := ownerIDs[5]
	eggID := primitive.NewObjectID()
	egg := domain.TeacherGiftBox{
		ID: eggID, UserID: eggOwner, MinimumRarity: "Common", Message: "meet your new friend",
		GrantedBy: ownerIDs[4], Status: "unopened", CreatedAt: time.Now().UTC(), RewardPool: giftbox.CharacterEggPool,
	}
	starterID := seenByOwner[eggOwner]
	if _, err := users.UpdateOne(ctx, bson.M{"_id": eggOwner}, bson.M{
		"$set":  bson.M{"equipped_character_id": starterID, "pinned_character_id": starterID},
		"$push": bson.M{"gift_boxes": egg},
	}); err != nil {
		t.Fatal(err)
	}
	eggService := character.NewEggService(characterRepository, character.SecurePicker{})
	eggGiftService := giftbox.NewService(giftRepository, nil, eggService)
	type hatchResult struct {
		characterID primitive.ObjectID
		err         error
	}
	hatches := make(chan hatchResult, 8)
	var hatchGroup sync.WaitGroup
	for range 8 {
		hatchGroup.Add(1)
		go func() {
			defer hatchGroup.Done()
			result, hatchErr := eggGiftService.Open(ctx, eggOwner.Hex(), eggID.Hex())
			if hatchErr != nil || result == nil || result.Character == nil {
				hatches <- hatchResult{err: hatchErr}
				return
			}
			hatches <- hatchResult{characterID: result.Character.ID}
		}()
	}
	hatchGroup.Wait()
	close(hatches)
	var revealedID primitive.ObjectID
	for result := range hatches {
		if result.err != nil || result.characterID.IsZero() {
			t.Fatalf("concurrent Egg open failed: %+v", result)
		}
		if revealedID.IsZero() {
			revealedID = result.characterID
		} else if revealedID != result.characterID {
			t.Fatalf("Egg rerolled under concurrency: first=%s next=%s", revealedID.Hex(), result.characterID.Hex())
		}
	}
	retry, err := eggGiftService.Open(ctx, eggOwner.Hex(), eggID.Hex())
	if err != nil || retry.Character == nil || retry.Character.ID != revealedID {
		t.Fatalf("Egg retry result = %+v, err=%v", retry, err)
	}
	origin := "character-egg:" + eggID.Hex()
	if count, err := database.Collection("baro_characters").CountDocuments(ctx, bson.M{"origin_key": origin}); err != nil || count != 1 {
		t.Fatalf("Egg origin count = %d, err=%v", count, err)
	}
	collection, err := characterService.Collection(ctx, eggOwner.Hex())
	if err != nil || len(collection) != 2 {
		t.Fatalf("Egg character collection = %+v, err=%v", collection, err)
	}
	ownership := character.NewOwnershipService(characterRepository, character.SecurePicker{})
	selection, err := ownership.Selection(ctx, eggOwner.Hex())
	if err != nil || selection.EquippedID != starterID.Hex() || selection.PinnedID != starterID.Hex() {
		t.Fatalf("Egg changed selection without consent: %+v, err=%v", selection, err)
	}
	selection, err = ownership.Equip(ctx, eggOwner.Hex(), revealedID.Hex())
	if err != nil || selection.EquippedID != revealedID.Hex() || selection.PinnedID != starterID.Hex() {
		t.Fatalf("explicit Egg equip = %+v, err=%v", selection, err)
	}
	if _, err := giftRepository.Transfer(ctx, eggID, eggOwner, ownerIDs[6]); !errors.Is(err, giftbox.ErrBoxNotFound) {
		t.Fatalf("opened Egg transferred: %v", err)
	}
}

func careEnergySnapshot(t *testing.T, ctx context.Context, users *mongo.Collection) migration.CareEnergySnapshot {
	t.Helper()
	cursor, err := users.Aggregate(ctx, migration.CareEnergySnapshotPipeline())
	if err != nil {
		t.Fatal(err)
	}
	defer cursor.Close(ctx)
	var snapshots []migration.CareEnergySnapshot
	if err := cursor.All(ctx, &snapshots); err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected one migration snapshot, got %d", len(snapshots))
	}
	return snapshots[0]
}
