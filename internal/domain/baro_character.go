package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrCharacterConflict = errors.New("character identity conflict")

type CharacterRarity string

const (
	CharacterNormal    CharacterRarity = "normal"
	CharacterMemeRare  CharacterRarity = "meme_rare"
	CharacterLegendary CharacterRarity = "legendary"
)

type CharacterDNA struct {
	Version     int             `bson:"version" json:"version"`
	Body        string          `bson:"body" json:"body"`
	Ears        string          `bson:"ears" json:"ears"`
	Eyes        string          `bson:"eyes" json:"eyes"`
	Mark        string          `bson:"mark" json:"mark"`
	Palette     string          `bson:"palette" json:"palette"`
	Pattern     string          `bson:"pattern" json:"pattern"`
	PatternSeed uint32          `bson:"pattern_seed" json:"pattern_seed"`
	Rarity      CharacterRarity `bson:"rarity" json:"rarity"`
}

func (dna CharacterDNA) Fingerprint() string {
	traits := []string{
		strconv.Itoa(dna.Version), dna.Body, dna.Ears, dna.Eyes,
		dna.Mark, dna.Palette, dna.Pattern, strconv.FormatUint(uint64(dna.PatternSeed), 10),
		string(dna.Rarity),
	}
	sum := sha256.Sum256([]byte(strings.Join(traits, ":")))
	return hex.EncodeToString(sum[:])
}

type BaroCharacter struct {
	ID          primitive.ObjectID `bson:"_id" json:"id"`
	OwnerID     primitive.ObjectID `bson:"owner_id" json:"owner_id"`
	Serial      string             `bson:"serial" json:"serial"`
	DNA         CharacterDNA       `bson:"dna" json:"dna"`
	Fingerprint string             `bson:"fingerprint" json:"fingerprint"`
	Source      string             `bson:"source" json:"source"`
	OriginKey   string             `bson:"origin_key,omitempty" json:"origin_key,omitempty"`
	IsStarter   bool               `bson:"is_starter" json:"is_starter"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
}

type CharacterSelection struct {
	EquippedID string `json:"equipped_id"`
	PinnedID   string `json:"pinned_id"`
}

type CharacterGrowth struct {
	BestStreak   int    `json:"best_streak"`
	FormIndex    int    `json:"form_index"`
	FormName     string `json:"form_name"`
	DetailIndex  int    `json:"detail_index"`
	NextDetailAt *int   `json:"next_detail_at"`
}

type CharacterGrowthSnapshot struct {
	CharacterGrowth
	CurrentStreak int    `json:"current_streak"`
	Mood          string `json:"mood"`
}

var characterFormMilestones = []int{0, 7, 30, 75}
var characterFormNames = []string{"tiny", "playful", "confident", "legendary"}
var characterDetailMilestones = []int{0, 1, 3, 7, 14, 21, 30, 50, 75, 100}

func ResolveCharacterGrowth(bestStreak int) CharacterGrowth {
	if bestStreak < 0 {
		bestStreak = 0
	}
	formIndex := 0
	for index, milestone := range characterFormMilestones {
		if bestStreak >= milestone {
			formIndex = index
		}
	}
	detailIndex := 0
	for index, milestone := range characterDetailMilestones {
		if bestStreak >= milestone {
			detailIndex = index
		}
	}
	var nextDetailAt *int
	if detailIndex+1 < len(characterDetailMilestones) {
		next := characterDetailMilestones[detailIndex+1]
		nextDetailAt = &next
	}
	return CharacterGrowth{
		BestStreak: bestStreak, FormIndex: formIndex, FormName: characterFormNames[formIndex],
		DetailIndex: detailIndex + 1, NextDetailAt: nextDetailAt,
	}
}
