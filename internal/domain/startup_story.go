package domain

import (
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrStartupRunActive = errors.New("you already have a startup running")
var ErrStartupNoActiveRun = errors.New("no startup is running")
var ErrStartupRunConflict = errors.New("your startup changed in another tab, please refresh")
var ErrStartupWrongStage = errors.New("that action isn't available right now")
var ErrStartupTooEarly = errors.New("your team is still building")
var ErrStartupInvalidChoice = errors.New("invalid choice")

const (
	StartupModeFree   = "free"
	StartupModeRanked = "ranked"

	StartupStatusActive = "active"
	StartupStatusEnded  = "ended"

	StartupStageFounder    = "founder"
	StartupStageHub        = "hub"
	StartupStageDeveloping = "developing"
	StartupStageEnded      = "ended"
)

type StartupStudio struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	OwnerID   primitive.ObjectID `bson:"owner_id" json:"owner_id"`
	Cohort    int                `bson:"cohort" json:"cohort"`
	Fame      int                `bson:"fame" json:"fame"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}

type StartupDev struct {
	ID       string `bson:"id" json:"id"`
	Name     string `bson:"name" json:"name"`
	Title    string `bson:"title" json:"title"`
	Sprite   string `bson:"sprite" json:"sprite"`
	Perk     string `bson:"perk,omitempty" json:"perk,omitempty"`
	Frontend int    `bson:"frontend" json:"frontend"`
	Backend  int    `bson:"backend" json:"backend"`
	Design   int    `bson:"design" json:"design"`
	Debug    int    `bson:"debug" json:"debug"`
	Salary   int    `bson:"salary" json:"salary"`
}

type StartupProject struct {
	Type      string    `bson:"type" json:"type"`
	Theme     string    `bson:"theme" json:"theme"`
	StaffIDs  []string  `bson:"staff_ids" json:"staff_ids"`
	StartedAt time.Time `bson:"started_at" json:"started_at"`
	EndsAt    time.Time `bson:"ends_at" json:"ends_at"`
}

type StartupReview struct {
	Reviewer string `bson:"reviewer" json:"reviewer"`
	Score    int    `bson:"score" json:"score"`
	Line     string `bson:"line" json:"line"`
}

type StartupResult struct {
	Type       string          `bson:"type" json:"type"`
	Theme      string          `bson:"theme" json:"theme"`
	Combo      string          `bson:"combo" json:"combo"`
	Reviews    []StartupReview `bson:"reviews" json:"reviews"`
	Total      int             `bson:"total" json:"total"`
	Bugs       int             `bson:"bugs" json:"bugs"`
	MoneyDelta int             `bson:"money_delta" json:"money_delta"`
	FansDelta  int             `bson:"fans_delta" json:"fans_delta"`
}

type StartupRun struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	OwnerID      primitive.ObjectID `bson:"owner_id" json:"owner_id"`
	Cohort       int                `bson:"cohort" json:"cohort"`
	Role         string             `bson:"role" json:"role"`
	Mode         string             `bson:"mode" json:"mode"`
	Seed         int64              `bson:"seed" json:"-"`
	Step         int64              `bson:"step" json:"-"`
	Status       string             `bson:"status" json:"status"`
	Outcome      string             `bson:"outcome,omitempty" json:"outcome,omitempty"`
	Stage        string             `bson:"stage" json:"stage"`
	ProjectIndex int                `bson:"project_index" json:"project_index"`
	Money        int                `bson:"money" json:"money"`
	Fans         int                `bson:"fans" json:"fans"`
	FounderOffer []StartupDev       `bson:"founder_offer,omitempty" json:"founder_offer,omitempty"`
	Staff        []StartupDev       `bson:"staff" json:"staff"`
	Project      *StartupProject    `bson:"project,omitempty" json:"project,omitempty"`
	LastResult   *StartupResult     `bson:"last_result,omitempty" json:"last_result,omitempty"`
	Score        int                `bson:"score" json:"score"`
	Version      int                `bson:"version" json:"version"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time          `bson:"updated_at" json:"updated_at"`
	EndedAt      *time.Time         `bson:"ended_at,omitempty" json:"ended_at,omitempty"`
}
