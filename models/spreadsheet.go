package models

type SpreadsheetData struct {
	ID             string `json:"ID"`
	JSDNumber      string `json:"JSDNumber"`
	FirstName      string `json:"FirstName"`
	LastName       string `json:"LastName"`
	Email          string `json:"Email"`
	CohortNumber   string `json:"CohortNumber"`
	Password       string `json:"Password"`
	Role           string `json:"Role"`
	ReflectionDay  string `json:"ReflectionDay"`
	ReflectionDate string `json:"ReflectionDate"`
	TechHappy      string `json:"TechHappy"`
	TechImprove    string `json:"TechImprove"`
	NonTechHappy   string `json:"NonTechHappy"`
	NonTechImprove string `json:"NonTechImprove"`
	Barometer      string `json:"Barometer"`
}