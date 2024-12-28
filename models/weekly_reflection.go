package models

// WeeklyReflection represents data for reflections grouped by week
type WeeklyReflection struct {
	ID     WeekID     `bson:"_id" json:"_id"`
	Users  []Reflection `bson:"users" json:"users"` // List of reflections for that week
}

// WeekID represents a combination of year and week
type WeekID struct {
	Week int `bson:"week" json:"week"`
	Year int `bson:"year" json:"year"`
}
