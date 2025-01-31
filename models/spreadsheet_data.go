package models

type SpreadsheetData struct {
	ID             string `json:"id"`
	JSDNumber      string `json:"jsdNumber"`
	FirstName      string `json:"firstName"`
	LastName       string `json:"lastName"`
	Email          string `json:"email"`
	CohortNumber   string `json:"cohortNumber"`
	Password       string `json:"password"`
	Role           string `json:"role"`
	ReflectionDay  string `json:"reflectionDay"`
	ReflectionDate string `json:"reflectionDate"`
	TechHappy      string `json:"techHappy"`
	TechImprove    string `json:"techImprove"`
	NonTechHappy   string `json:"nonTechHappy"`
	NonTechImprove string `json:"nonTechImprove"`
	Barometer      string `json:"barometer"`
}