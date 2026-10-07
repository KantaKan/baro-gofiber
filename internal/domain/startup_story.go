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
var ErrStartupTeamFull = errors.New("no free desk: buy one in the Office tab")
var ErrStartupDeskLimit = errors.New("no room for another desk this act")
var ErrStartupNoFunds = errors.New("not enough money to hire")
var ErrStartupNoAttempts = errors.New("no ranked attempts left this week")

const (
	StartupModeFree   = "free"
	StartupModeRanked = "ranked"

	StartupStatusActive = "active"
	StartupStatusEnded  = "ended"

	StartupStageFounder    = "founder"
	StartupStageHub        = "hub"
	StartupStageDeveloping = "developing"
	StartupStageItem       = "item"
	StartupStagePerk       = "perk"
	StartupStageEvent      = "event"
	StartupStageIPOChoice  = "ipo_choice"
	StartupStageEnded      = "ended"

	StartupOutcomeIPO   = "ipo"
	StartupOutcomePivot = "pivot"

	StartupBoardWeekly  = "weekly"
	StartupBoardDeepest = "deepest"
	StartupBoardFame    = "fame"

	StartupLeaderboardLimit = 100
)

type StartupLeaderboardEntry struct {
	OwnerID   primitive.ObjectID `bson:"owner_id" json:"owner_id"`
	Name      string             `bson:"name" json:"name"`
	Score     int                `bson:"score,omitempty" json:"score,omitempty"`
	Fame      int                `bson:"fame,omitempty" json:"fame,omitempty"`
	MaxAct    int                `bson:"max_act,omitempty" json:"max_act,omitempty"`
	BestRunID primitive.ObjectID `bson:"best_run_id,omitempty" json:"best_run_id,omitempty"`
	Outcome   string             `bson:"outcome,omitempty" json:"outcome,omitempty"`
}

type StartupStudio struct {
	ID               primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	OwnerID          primitive.ObjectID `bson:"owner_id" json:"owner_id"`
	Cohort           int                `bson:"cohort" json:"cohort"`
	Fame             int                `bson:"fame" json:"fame"`
	DiscoveredCombos []string           `bson:"discovered_combos,omitempty" json:"discovered_combos,omitempty"`
	UnlockedFounders []string           `bson:"unlocked_founders,omitempty" json:"unlocked_founders,omitempty"`
	UnlockedItems    []string           `bson:"unlocked_items,omitempty" json:"unlocked_items,omitempty"`
	OfficeSkin       string             `bson:"office_skin,omitempty" json:"office_skin,omitempty"`
	HallOfFame       []StartupHallEntry `bson:"hall_of_fame,omitempty" json:"hall_of_fame,omitempty"`
	OSSShips         int                `bson:"oss_ships,omitempty" json:"-"`
	CreatedAt        time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt        time.Time          `bson:"updated_at" json:"updated_at"`
}

type StartupHallEntry struct {
	RunID   primitive.ObjectID `bson:"run_id" json:"run_id"`
	Mode    string             `bson:"mode" json:"mode"`
	WeekKey string             `bson:"week_key,omitempty" json:"week_key,omitempty"`
	Score   int                `bson:"score" json:"score"`
	Outcome string             `bson:"outcome" json:"outcome"`
	Founder string             `bson:"founder" json:"founder"`
	EndedAt time.Time          `bson:"ended_at" json:"ended_at"`
}

type StartupMarket struct {
	Hot  []string `bson:"hot" json:"hot"`
	Cold []string `bson:"cold,omitempty" json:"cold,omitempty"`
}

type StartupGenmate struct {
	UserID primitive.ObjectID `bson:"user_id" json:"-"`
	Name   string             `bson:"name" json:"-"`
}

type StartupDev struct {
	ID        string   `bson:"id" json:"id"`
	Name      string   `bson:"name" json:"name"`
	Title     string   `bson:"title" json:"title"`
	Role      string   `bson:"role,omitempty" json:"role,omitempty"`
	GenmateID string   `bson:"genmate_id,omitempty" json:"genmate_id,omitempty"`
	Sprite    string   `bson:"sprite" json:"sprite"`
	Perk      string   `bson:"perk,omitempty" json:"perk,omitempty"`
	Trait     string   `bson:"trait,omitempty" json:"trait,omitempty"`
	Frontend  int      `bson:"frontend" json:"frontend"`
	Backend   int      `bson:"backend" json:"backend"`
	Design    int      `bson:"design" json:"design"`
	Debug     int      `bson:"debug" json:"debug"`
	Salary    int      `bson:"salary" json:"salary"`
	Level     int      `bson:"level,omitempty" json:"level,omitempty"`
	XP        int      `bson:"xp,omitempty" json:"xp,omitempty"`
	Burnout   int      `bson:"burnout,omitempty" json:"burnout,omitempty"`
	Perks     []string `bson:"perks,omitempty" json:"perks,omitempty"`
	XPNext    int      `bson:"xp_next,omitempty" json:"xp_next,omitempty"`
}

type StartupPitch struct {
	Type  string `bson:"type" json:"type"`
	Theme string `bson:"theme" json:"theme"`
	Title string `bson:"title" json:"title"`
}

type StartupPendingPerk struct {
	DevID string   `bson:"dev_id" json:"dev_id"`
	Offer []string `bson:"offer" json:"offer"`
}

type StartupPendingEvent struct {
	ID      string   `bson:"id" json:"id"`
	Options []string `bson:"options" json:"options"`
}

type StartupProject struct {
	Type      string    `bson:"type" json:"type"`
	Theme     string    `bson:"theme" json:"theme"`
	Boss      string    `bson:"boss,omitempty" json:"boss,omitempty"`
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
	ID            primitive.ObjectID   `bson:"_id,omitempty" json:"_id"`
	OwnerID       primitive.ObjectID   `bson:"owner_id" json:"owner_id"`
	Cohort        int                  `bson:"cohort" json:"cohort"`
	Role          string               `bson:"role" json:"role"`
	Mode          string               `bson:"mode" json:"mode"`
	WeekKey       string               `bson:"week_key,omitempty" json:"week_key,omitempty"`
	Founder       string               `bson:"founder,omitempty" json:"founder,omitempty"`
	Seed          int64                `bson:"seed" json:"-"`
	Step          int64                `bson:"step" json:"-"`
	Status        string               `bson:"status" json:"status"`
	Outcome       string               `bson:"outcome,omitempty" json:"outcome,omitempty"`
	Stage         string               `bson:"stage" json:"stage"`
	Act           int                  `bson:"act" json:"act"`
	Market        StartupMarket        `bson:"market" json:"market"`
	GenmatePool   []StartupGenmate     `bson:"genmate_pool,omitempty" json:"-"`
	BossOrder     []string             `bson:"boss_order,omitempty" json:"boss_order,omitempty"`
	BossesPassed  int                  `bson:"bosses_passed" json:"bosses_passed"`
	ProjectIndex  int                  `bson:"project_index" json:"project_index"`
	Money         int                  `bson:"money" json:"money"`
	Fans          int                  `bson:"fans" json:"fans"`
	FounderOffer  []StartupDev         `bson:"founder_offer,omitempty" json:"founder_offer,omitempty"`
	Staff         []StartupDev         `bson:"staff" json:"staff"`
	Desks         []int                `bson:"desks,omitempty" json:"desks"`
	DeskLimit     int                  `bson:"-" json:"desk_limit"`
	Candidates    []StartupDev         `bson:"candidates,omitempty" json:"candidates,omitempty"`
	Items         []string             `bson:"items,omitempty" json:"items,omitempty"`
	ItemOffer     []string             `bson:"item_offer,omitempty" json:"item_offer,omitempty"`
	UnlockedItems []string             `bson:"unlocked_items,omitempty" json:"-"`
	Project       *StartupProject      `bson:"project,omitempty" json:"project,omitempty"`
	LastResult    *StartupResult       `bson:"last_result,omitempty" json:"last_result,omitempty"`
	Score         int                  `bson:"score" json:"score"`
	Version       int                  `bson:"version" json:"version"`
	CreatedAt     time.Time            `bson:"created_at" json:"created_at"`
	UpdatedAt     time.Time            `bson:"updated_at" json:"updated_at"`
	EndedAt       *time.Time           `bson:"ended_at,omitempty" json:"ended_at,omitempty"`
	MaxAct        int                  `bson:"max_act,omitempty" json:"max_act,omitempty"`
	Endless       bool                 `bson:"endless,omitempty" json:"endless,omitempty"`
	OSS           bool                 `bson:"oss,omitempty" json:"oss,omitempty"`
	Pitches       []StartupPitch       `bson:"pitches,omitempty" json:"pitches,omitempty"`
	WorldEvent    string               `bson:"world_event,omitempty" json:"world_event,omitempty"`
	BossGimmick   string               `bson:"boss_gimmick,omitempty" json:"boss_gimmick,omitempty"`
	PendingPerk   *StartupPendingPerk  `bson:"pending_perk,omitempty" json:"pending_perk,omitempty"`
	PerkQueue     []string             `bson:"perk_queue,omitempty" json:"-"`
	PendingEvent  *StartupPendingEvent `bson:"pending_event,omitempty" json:"pending_event,omitempty"`
	ResumeStage   string               `bson:"resume_stage,omitempty" json:"-"`
	NextBoss      string               `bson:"next_boss,omitempty" json:"next_boss,omitempty"`
	NextPassMark  int                  `bson:"next_pass_mark,omitempty" json:"next_pass_mark,omitempty"`
	Log           []string             `bson:"log,omitempty" json:"log,omitempty"`
}
