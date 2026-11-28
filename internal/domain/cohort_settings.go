package domain

// CohortSettings holds per-cohort check-in configuration.
type CohortSettings struct {
	CohortNumber    int               `bson:"cohort_number" json:"cohort_number"`
	Rules           CheckinRules      `bson:"rules" json:"rules"`
	AllowedNetworks []string          `bson:"allowed_networks" json:"allowed_networks"`
	GroupChatURLs   map[string]string `bson:"group_chat_urls,omitempty" json:"group_chat_urls,omitempty"`
	StartDate       string            `bson:"start_date,omitempty" json:"start_date,omitempty"`
	CourseDays      int               `bson:"course_days,omitempty" json:"course_days,omitempty"`
	// GraduatedOn is the cohort's last class day (YYYY-MM-DD); after it the cohort has no class.
	GraduatedOn string `bson:"graduated_on,omitempty" json:"graduated_on,omitempty"`
	// CheckinRegion: "anywhere" (default) or "thailand" (uses the proxy's country header; no header = allowed).
	CheckinRegion string `bson:"checkin_region,omitempty" json:"checkin_region,omitempty"`
	// FlagUnusual marks shared-network check-ins on the admin board (nil = on).
	FlagUnusual *bool `bson:"flag_unusual,omitempty" json:"flag_unusual,omitempty"`
	// ChessEnabled runs the cohort's team chess game.
	ChessEnabled bool `bson:"chess_enabled,omitempty" json:"chess_enabled,omitempty"`
}

// InSession reports whether the cohort still has class on date (YYYY-MM-DD): true until it graduates.
func (s CohortSettings) InSession(date string) bool {
	return s.GraduatedOn == "" || date <= s.GraduatedOn
}

// FlagsOn reports whether unusual check-ins are flagged (default on).
func (s CohortSettings) FlagsOn() bool { return s.FlagUnusual == nil || *s.FlagUnusual }

type CohortSettingsRepository interface {
	// FindByCohort returns (nil, nil) when the cohort has no stored settings.
	FindByCohort(ctx interface{}, cohort int) (*CohortSettings, error)
	Upsert(ctx interface{}, s *CohortSettings) error
}
