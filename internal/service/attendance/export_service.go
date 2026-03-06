package attendance

import (
	"bytes"
	"context"
	"encoding/csv"
	"strings"

	"gofiber-baro/internal/domain"
	userService "gofiber-baro/internal/service/user"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ExportService builds Salesforce-compatible CSV exports.
type ExportService struct {
	recordRepo  domain.AttendanceRepository
	userService *userService.Service
}

func NewExportService(recordRepo domain.AttendanceRepository, us *userService.Service) *ExportService {
	return &ExportService{recordRepo: recordRepo, userService: us}
}

// salesforceStatus maps internal attendance status → Salesforce display label.
// Worst session in the day wins: Absent > AbsentExcused > Late > LateExcused > Present > NoClass > Holiday
// Dropout and Dismissed return empty string (blank in export)
func salesforceStatus(morning, afternoon domain.AttendanceStatus) string {
	combined := []domain.AttendanceStatus{morning, afternoon}

	hasAbsent := false
	hasAbsentExcused := false
	hasLate := false
	hasLateExcused := false
	hasPresent := false
	hasNoClass := false
	hasHoliday := false

	for _, s := range combined {
		switch s {
		case domain.StatusAbsent:
			hasAbsent = true
		case domain.StatusAbsentExcused:
			hasAbsentExcused = true
		case domain.StatusLate:
			hasLate = true
		case domain.StatusLateExcused:
			hasLateExcused = true
		case domain.StatusPresent:
			hasPresent = true
		case domain.StatusNoClass:
			hasNoClass = true
		case domain.StatusHoliday:
			hasHoliday = true
		}
	}

	switch {
	case hasAbsent:
		return "Absent"
	case hasAbsentExcused:
		return "Absent - Excused"
	case hasLate:
		return "Late"
	case hasLateExcused:
		return "Late - Excused"
	case hasPresent:
		return "Present"
	case hasNoClass:
		return "No Class"
	case hasHoliday:
		return "Holiday"
	default:
		// dropout, dismissed, or no record = blank
		return ""
	}
}

// ExportSalesforceCSV generates a Salesforce-compatible CSV for the given cohort and date range.
// Each row = one student × one day, sorted by date then JSD number.
func (s *ExportService) ExportSalesforceCSV(cohort int, startDate, endDate string) ([]byte, error) {
	ctx := context.Background()

	// 1. Fetch all learners in the cohort, sorted by jsd_number.
	users, _, err := s.userService.GetAllUsers(cohort, "learner", "", "", "jsd_number", 1, 1, 1000)
	if err != nil {
		return nil, err
	}

	// 2. Fetch attendance records filtered by cohort AND date range directly in MongoDB.
	//    bson.D (ordered slice) is required for sort — plain map causes a driver panic.
	findOpts := options.Find().SetSort(bson.D{
		{Key: "date", Value: 1},
		{Key: "session", Value: 1},
	})

	bsonFilter := bson.M{
		"cohort_number": cohort,
		"deleted":       bson.M{"$ne": true},
		"date":          bson.M{"$gte": startDate, "$lte": endDate},
	}

	records, err := s.recordRepo.FindRecordsRaw(ctx, bsonFilter, findOpts)
	if err != nil {
		return nil, err
	}

	// 3. Build a lookup: (userID, date, session) → status
	type sessionKey struct {
		userID  string
		date    string
		session domain.AttendanceSession
	}
	sessionMap := make(map[sessionKey]domain.AttendanceStatus)
	dateSet := make(map[string]struct{})

	for _, r := range records {
		key := sessionKey{r.UserID.Hex(), r.Date, r.Session}
		sessionMap[key] = r.Status
		dateSet[r.Date] = struct{}{}
	}

	dates := sortedKeys(dateSet)

	// 4. Build CSV.
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// Header exactly as Salesforce expects.
	_ = w.Write([]string{"Learner ID", "First Name", "Last Name", "Date [YYYY-MM-DD]", "Attendance status", "Notes"})

	for _, date := range dates {
		for _, u := range users {
			uid := u.ID.Hex()
			morning := sessionMap[sessionKey{uid, date, domain.SessionMorning}]
			afternoon := sessionMap[sessionKey{uid, date, domain.SessionAfternoon}]

			// If user is dropout/dismissed, leave status blank (they still appear in CSV)
			status := ""
			if u.AttendanceStatus != "dropout" && u.AttendanceStatus != "dismissed" {
				if morning != "" || afternoon != "" {
					status = salesforceStatus(morning, afternoon)
				}
			}

			_ = w.Write([]string{
				u.SalesforceID,
				strings.TrimSpace(u.FirstName),
				strings.TrimSpace(u.LastName),
				date,
				status,
				"", // Notes always blank per requirement
			})
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// sortedKeys returns a chronologically sorted slice of YYYY-MM-DD date strings.
func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Insertion sort — YYYY-MM-DD strings are lexicographically chronological.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
