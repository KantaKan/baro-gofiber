package domain

// CohortSettings holds per-cohort check-in configuration.
type CohortSettings struct {
	CohortNumber    int               `bson:"cohort_number" json:"cohort_number"`
	Rules           CheckinRules      `bson:"rules" json:"rules"`
	AllowedNetworks []string          `bson:"allowed_networks" json:"allowed_networks"`
	GroupChatURLs   map[string]string `bson:"group_chat_urls,omitempty" json:"group_chat_urls,omitempty"`
	StartDate       string            `bson:"start_date,omitempty" json:"start_date,omitempty"`
	CourseDays      int               `bson:"course_days,omitempty" json:"course_days,omitempty"`
}

type CohortSettingsRepository interface {
	// FindByCohort returns (nil, nil) when the cohort has no stored settings.
	FindByCohort(ctx interface{}, cohort int) (*CohortSettings, error)
	Upsert(ctx interface{}, s *CohortSettings) error
}
