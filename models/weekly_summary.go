package models

import "time"

type StudentInfo struct {
	UserID    string    `bson:"user_id" json:"user_id"`
	FirstName string    `bson:"first_name" json:"first_name"`
	LastName  string    `bson:"last_name" json:"last_name"`
	ZoomName  string    `bson:"zoom_name" json:"zoom_name"`
	JsdNumber string    `bson:"jsd_number" json:"jsd_number"`
	Barometer string    `bson:"barometer" json:"barometer"`
	Date      time.Time `bson:"date" json:"date"`
}

type WeeklySummary struct {
	WeekStartDate      string        `json:"week_start_date"`
	WeekEndDate        string        `json:"week_end_date"`
	StressedStudents   []StudentInfo `json:"stressed_students"`
	OverwhelmedStudents []StudentInfo `json:"overwhelmed_students"`
}