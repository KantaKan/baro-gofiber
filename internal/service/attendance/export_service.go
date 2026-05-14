package attendance

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"sort"
	"strings"
	"time"

	"gofiber-baro/internal/domain"
	userService "gofiber-baro/internal/service/user"

	"github.com/xuri/excelize/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ExportFormat string

const (
	ExportFormatCSV  ExportFormat = "csv"
	ExportFormatXLSX ExportFormat = "xlsx"
)

type ExportStructure string

const (
	ExportStructureDaily   ExportStructure = "daily"
	ExportStructureSummary ExportStructure = "summary"
	ExportStructureWeekly  ExportStructure = "weekly"
)

type ExportRequest struct {
	Cohort     int
	StartDate  string
	EndDate    string
	Format     ExportFormat
	Structure  ExportStructure
	SplitAMPM  bool
	LeaveData  []domain.LeaveRequest
	StatusFilter string // "", "active", "dropout", "dismissed"
}

type ExportService struct {
	recordRepo  domain.AttendanceRepository
	userService *userService.Service
}

func NewExportService(recordRepo domain.AttendanceRepository, us *userService.Service) *ExportService {
	return &ExportService{recordRepo: recordRepo, userService: us}
}

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
		return ""
	}
}

type sessionKey struct {
	userID  string
	date    string
	session domain.AttendanceSession
}

func buildAttendanceLookup(records []domain.AttendanceRecord) map[sessionKey]domain.AttendanceStatus {
	m := make(map[sessionKey]domain.AttendanceStatus)
	for _, r := range records {
		key := sessionKey{r.UserID.Hex(), r.Date, r.Session}
		m[key] = r.Status
	}
	return m
}

func isExcludedAttendanceStatus(status string) bool {
	return status == "dropout" || status == "dismissed"
}

func Export(req ExportRequest, recordRepo domain.AttendanceRepository, userService *userService.Service) ([]byte, string, error) {
	ctx := context.Background()

	users, _, err := userService.GetAllUsers(req.Cohort, "learner", "", "", "first_name", 1, 1, 5000)
	if err != nil {
		return nil, "", fmt.Errorf("fetch users: %w", err)
	}

	findOpts := options.Find().SetSort(bson.D{
		{Key: "date", Value: 1},
		{Key: "session", Value: 1},
	})

	bsonFilter := bson.M{
		"cohort_number": req.Cohort,
		"deleted":       bson.M{"$ne": true},
		"date":          bson.M{"$gte": req.StartDate, "$lte": req.EndDate},
	}

	records, err := recordRepo.FindRecordsRaw(ctx, bsonFilter, findOpts)
	if err != nil {
		return nil, "", fmt.Errorf("fetch records: %w", err)
	}

	lookup := buildAttendanceLookup(records)

	dateSet := make(map[string]struct{})
	for _, r := range records {
		dateSet[r.Date] = struct{}{}
	}
	dates := sortedKeys(dateSet)

	filteredUsers := filterByStatus(users, req.StatusFilter)

	switch req.Structure {
	case ExportStructureSummary:
		return exportSummary(req, filteredUsers, lookup, dates)
	case ExportStructureWeekly:
		return exportWeekly(req, filteredUsers, lookup, dates)
	default:
		return exportDaily(req, filteredUsers, lookup, dates)
	}
}

func filterByStatus(users []domain.User, statusFilter string) []domain.User {
	if statusFilter == "" || statusFilter == "all" {
		return users
	}
	var filtered []domain.User
	for _, u := range users {
		switch statusFilter {
		case "active":
			if !isExcludedAttendanceStatus(u.AttendanceStatus) {
				filtered = append(filtered, u)
			}
		case "dropout":
			if u.AttendanceStatus == "dropout" {
				filtered = append(filtered, u)
			}
		case "dismissed":
			if u.AttendanceStatus == "dismissed" {
				filtered = append(filtered, u)
			}
		}
	}
	return filtered
}

func dailyStatusFromLookup(u domain.User, date string, lookup map[sessionKey]domain.AttendanceStatus, splitAMPM bool) (string, string, string) {
	morning := lookup[sessionKey{u.ID.Hex(), date, domain.SessionMorning}]
	afternoon := lookup[sessionKey{u.ID.Hex(), date, domain.SessionAfternoon}]

	status := ""
	amStatus := ""
	pmStatus := ""

	if isExcludedAttendanceStatus(u.AttendanceStatus) {
		return amStatus, pmStatus, status
	}

	if morning != "" || afternoon != "" {
		status = salesforceStatus(morning, afternoon)
	}
	amStatus = string(morning)
	pmStatus = string(afternoon)

	return amStatus, pmStatus, status
}

func formatStatusLabel(status string) string {
	switch status {
	case "present":
		return "Present"
	case "late":
		return "Late"
	case "absent":
		return "Absent"
	case "late_excused":
		return "Late - Excused"
	case "absent_excused":
		return "Absent - Excused"
	case "no_class":
		return "No Class"
	case "holiday":
		return "Holiday"
	default:
		return ""
	}
}

// ---- DAILY STRUCTURE ----

func dailyHeaders(splitAMPM bool, includeLeave bool) []string {
	if splitAMPM {
		if includeLeave {
			return []string{"Learner ID", "First Name", "Last Name", "JSD Number", "Cohort", "Date", "AM Status", "PM Status", "Daily Status", "On Leave", "Notes"}
		}
		return []string{"Learner ID", "First Name", "Last Name", "JSD Number", "Cohort", "Date", "AM Status", "PM Status", "Daily Status", "Notes"}
	}
	if includeLeave {
		return []string{"Learner ID", "First Name", "Last Name", "Date", "Attendance Status", "On Leave", "Notes"}
	}
	return []string{"Learner ID", "First Name", "Last Name", "Date", "Attendance Status", "Notes"}
}

func dailyRows(users []domain.User, dates []string, lookup map[sessionKey]domain.AttendanceStatus, splitAMPM bool, leaveLookup map[string]map[string]bool) [][]string {
	var rows [][]string
	for _, date := range dates {
		for _, u := range users {
			uid := u.ID.Hex()
			amStatus, pmStatus, dailyStatus := dailyStatusFromLookup(u, date, lookup, splitAMPM)

			onLeave := ""
			if leaveLookup != nil {
				if userLeaves, ok := leaveLookup[uid]; ok {
					if userLeaves[date] {
						onLeave = "Yes"
					}
				}
			}

			if splitAMPM {
				rows = append(rows, []string{
					u.SalesforceID,
					strings.TrimSpace(u.FirstName),
					strings.TrimSpace(u.LastName),
					strings.TrimSpace(u.JSDNumber),
					fmt.Sprintf("%d", u.CohortNumber),
					date,
					formatStatusLabel(amStatus),
					formatStatusLabel(pmStatus),
					dailyStatus,
					onLeave,
					"",
				})
			} else {
				rows = append(rows, []string{
					u.SalesforceID,
					strings.TrimSpace(u.FirstName),
					strings.TrimSpace(u.LastName),
					date,
					dailyStatus,
					"",
				})
			}
		}
	}
	return rows
}

func dailyRowsWithLeave(users []domain.User, dates []string, lookup map[sessionKey]domain.AttendanceStatus, splitAMPM bool, leaveLookup map[string]map[string]bool) [][]string {
	var rows [][]string
	for _, date := range dates {
		for _, u := range users {
			uid := u.ID.Hex()
			amStatus, pmStatus, dailyStatus := dailyStatusFromLookup(u, date, lookup, splitAMPM)

			onLeave := ""
			if leaveLookup != nil {
				if userLeaves, ok := leaveLookup[uid]; ok {
					if userLeaves[date] {
						onLeave = "Yes"
					}
				}
			}

			if splitAMPM {
				rows = append(rows, []string{
					u.SalesforceID,
					strings.TrimSpace(u.FirstName),
					strings.TrimSpace(u.LastName),
					strings.TrimSpace(u.JSDNumber),
					fmt.Sprintf("%d", u.CohortNumber),
					date,
					formatStatusLabel(amStatus),
					formatStatusLabel(pmStatus),
					dailyStatus,
					onLeave,
					"",
				})
			} else {
				rows = append(rows, []string{
					u.SalesforceID,
					strings.TrimSpace(u.FirstName),
					strings.TrimSpace(u.LastName),
					date,
					dailyStatus,
					onLeave,
					"",
				})
			}
		}
	}
	return rows
}

// ---- SUMMARY STRUCTURE ----

type userSummary struct {
	present       int
	late          int
	absent        int
	lateExcused   int
	absentExcused int
	presentDays   int
	absentDays    int
}

func buildSummaries(users []domain.User, lookup map[sessionKey]domain.AttendanceStatus) map[string]*userSummary {
	summaries := make(map[string]*userSummary)
	userDates := make(map[string]map[string]struct {
		morning   string
		afternoon string
	})

	for _, u := range users {
		uid := u.ID.Hex()
		summaries[uid] = &userSummary{}
		userDates[uid] = make(map[string]struct {
			morning   string
			afternoon string
		})
	}

	for key, status := range lookup {
		uid := key.userID
		if _, ok := summaries[uid]; !ok {
			continue
		}
		us := summaries[uid]
		switch status {
		case domain.StatusPresent:
			us.present++
		case domain.StatusLate:
			us.late++
		case domain.StatusAbsent:
			us.absent++
		case domain.StatusLateExcused:
			us.lateExcused++
		case domain.StatusAbsentExcused:
			us.absentExcused++
		}
		dateData := userDates[uid][key.date]
		if key.session == domain.SessionMorning {
			dateData.morning = string(status)
		} else {
			dateData.afternoon = string(status)
		}
		userDates[uid][key.date] = dateData
	}

	for uid, dates := range userDates {
		us := summaries[uid]
		for _, dateData := range dates {
			hasAbsent := dateData.morning == "absent" || dateData.afternoon == "absent"
			hasPresent := dateData.morning == "present" || dateData.afternoon == "present"
			hasLate := dateData.morning == "late" || dateData.afternoon == "late"
			hasLateExcused := dateData.morning == "late_excused" || dateData.afternoon == "late_excused"

			if hasAbsent {
				us.absentDays++
			} else if hasPresent || hasLate || hasLateExcused {
				us.presentDays++
			}
		}
	}

	return summaries
}

func warningLevel(absent int) string {
	if absent >= 7 {
		return "red"
	}
	if absent >= 4 {
		return "yellow"
	}
	return "normal"
}

func summaryHeaders() []string {
	return []string{"Learner ID", "First Name", "Last Name", "JSD Number", "Cohort", "Total Present", "Total Late", "Total Absent", "Late Excused", "Absent Excused", "Present Days", "Absent Days", "Attendance Rate %", "Warning Level"}
}

func summaryRows(users []domain.User, summaries map[string]*userSummary) [][]string {
	var rows [][]string
	for _, u := range users {
		uid := u.ID.Hex()
		us := summaries[uid]
		if us == nil {
			us = &userSummary{}
		}
		total := us.present + us.late + us.absent + us.lateExcused + us.absentExcused
		rate := 0.0
		if total > 0 {
			attended := us.present + us.late + us.lateExcused
			rate = float64(attended) / float64(total) * 100
		}

		rows = append(rows, []string{
			u.SalesforceID,
			strings.TrimSpace(u.FirstName),
			strings.TrimSpace(u.LastName),
			strings.TrimSpace(u.JSDNumber),
			fmt.Sprintf("%d", u.CohortNumber),
			fmt.Sprintf("%d", us.present),
			fmt.Sprintf("%d", us.late),
			fmt.Sprintf("%d", us.absent),
			fmt.Sprintf("%d", us.lateExcused),
			fmt.Sprintf("%d", us.absentExcused),
			fmt.Sprintf("%d", us.presentDays),
			fmt.Sprintf("%d", us.absentDays),
			fmt.Sprintf("%.1f", rate),
			warningLevel(us.absent),
		})
	}
	return rows
}

// ---- WEEKLY STRUCTURE ----

type weekRange struct {
	Start string
	End   string
}

func groupByWeek(dates []string) []weekRange {
	if len(dates) == 0 {
		return nil
	}

	var weeks []weekRange
	currentStart := dates[0]
	currentEnd := dates[0]

	for i := 1; i < len(dates); i++ {
		prev, _ := time.Parse("2006-01-02", dates[i-1])
		curr, _ := time.Parse("2006-01-02", dates[i])
		if curr.Sub(prev).Hours() > 24 || dates[i][:4] != dates[i-1][:4] || getISOWeek(dates[i]) != getISOWeek(dates[i-1]) {
			weeks = append(weeks, weekRange{Start: currentStart, End: currentEnd})
			currentStart = dates[i]
		}
		currentEnd = dates[i]
	}
	weeks = append(weeks, weekRange{Start: currentStart, End: currentEnd})

	return weeks
}

func getISOWeek(dateStr string) int {
	t, _ := time.Parse("2006-01-02", dateStr)
	_, week := t.ISOWeek()
	return week
}

func weeklyHeaders(weekRanges []weekRange) []string {
	headers := []string{"Learner ID", "First Name", "Last Name", "JSD Number", "Cohort"}
	for _, w := range weekRanges {
		headers = append(headers, fmt.Sprintf("%s - %s", w.Start, w.End))
	}
	headers = append(headers, "Total Present", "Total Absent")
	return headers
}

func weeklyRows(users []domain.User, lookup map[sessionKey]domain.AttendanceStatus, weekRanges []weekRange) [][]string {
	var rows [][]string
	for _, u := range users {
		uid := u.ID.Hex()
		row := []string{
			u.SalesforceID,
			strings.TrimSpace(u.FirstName),
			strings.TrimSpace(u.LastName),
			strings.TrimSpace(u.JSDNumber),
			fmt.Sprintf("%d", u.CohortNumber),
		}

		totalAbsent := 0
		totalPresent := 0

		for _, w := range weekRanges {
			weekAbsent := 0
			weekPresent := 0

			start, _ := time.Parse("2006-01-02", w.Start)
			end, _ := time.Parse("2006-01-02", w.End)
			for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
				dateStr := d.Format("2006-01-02")
				morning := lookup[sessionKey{uid, dateStr, domain.SessionMorning}]
				afternoon := lookup[sessionKey{uid, dateStr, domain.SessionAfternoon}]
				if isExcludedAttendanceStatus(u.AttendanceStatus) {
					continue
				}
				if morning == domain.StatusAbsent || afternoon == domain.StatusAbsent {
					weekAbsent++
				} else if morning == domain.StatusPresent || afternoon == domain.StatusPresent ||
					morning == domain.StatusLate || afternoon == domain.StatusLate {
					weekPresent++
				}
			}
			summaryLabel := fmt.Sprintf("P%d/A%d", weekPresent, weekAbsent)
			row = append(row, summaryLabel)
			totalAbsent += weekAbsent
			totalPresent += weekPresent
		}

		row = append(row, fmt.Sprintf("%d", totalPresent), fmt.Sprintf("%d", totalAbsent))
		rows = append(rows, row)
	}
	return rows
}

// ---- CSV OUTPUT ----

func writeCSV(headers []string, rows [][]string) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(headers); err != nil {
		return nil, err
	}
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

// ---- EXCEL OUTPUT ----

func writeXLSX(headers []string, rows [][]string) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Sheet1"
	style, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"4472C4"}},
	})
	f.SetCellStyle(sheet, "A1", cellRef(len(headers), 1), style)

	for col, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	for rowIdx, row := range rows {
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			f.SetCellValue(sheet, cell, val)
		}
	}

	for col := 0; col < len(headers); col++ {
		colLetter, _ := excelize.ColumnNumberToName(col + 1)
		f.SetColWidth(sheet, colLetter, colLetter, 18)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func cellRef(cols int, row int) string {
	colLetter, _ := excelize.ColumnNumberToName(cols)
	return fmt.Sprintf("%s%d", colLetter, row)
}

// ---- EXPORT BY STRUCTURE ----

func exportDaily(req ExportRequest, users []domain.User, lookup map[sessionKey]domain.AttendanceStatus, dates []string) ([]byte, string, error) {
	leaveLookup := buildLeaveLookup(req.LeaveData)

	var rows [][]string
	var headers []string

	if req.SplitAMPM {
		if len(req.LeaveData) > 0 {
			headers = []string{"Learner ID", "First Name", "Last Name", "JSD Number", "Cohort", "Date", "AM Status", "PM Status", "Daily Status", "On Leave", "Notes"}
			rows = dailyRowsWithLeave(users, dates, lookup, true, leaveLookup)
		} else {
			headers = []string{"Learner ID", "First Name", "Last Name", "JSD Number", "Cohort", "Date", "AM Status", "PM Status", "Daily Status", "Notes"}
			rows = dailyRows(users, dates, lookup, true, nil)
		}
	} else {
		if len(req.LeaveData) > 0 {
			headers = []string{"Learner ID", "First Name", "Last Name", "Date", "Attendance Status", "On Leave", "Notes"}
			rows = dailyRowsWithLeave(users, dates, lookup, false, leaveLookup)
		} else {
			headers = []string{"Learner ID", "First Name", "Last Name", "Date", "Attendance Status", "Notes"}
			rows = dailyRows(users, dates, lookup, false, nil)
		}
	}

	ext := "csv"
	switch req.Format {
	case ExportFormatXLSX:
		ext = "xlsx"
		data, err := writeXLSX(headers, rows)
		return data, ext, err
	default:
		data, err := writeCSV(headers, rows)
		return data, ext, err
	}
}

func exportSummary(req ExportRequest, users []domain.User, lookup map[sessionKey]domain.AttendanceStatus, _ []string) ([]byte, string, error) {
	summaries := buildSummaries(users, lookup)
	headers := summaryHeaders()
	rows := summaryRows(users, summaries)

	ext := "csv"
	switch req.Format {
	case ExportFormatXLSX:
		ext = "xlsx"
		data, err := writeXLSX(headers, rows)
		return data, ext, err
	default:
		data, err := writeCSV(headers, rows)
		return data, ext, err
	}
}

func exportWeekly(req ExportRequest, users []domain.User, lookup map[sessionKey]domain.AttendanceStatus, dates []string) ([]byte, string, error) {
	weeks := groupByWeek(dates)
	headers := weeklyHeaders(weeks)
	rows := weeklyRows(users, lookup, weeks)

	ext := "csv"
	switch req.Format {
	case ExportFormatXLSX:
		ext = "xlsx"
		data, err := writeXLSX(headers, rows)
		return data, ext, err
	default:
		data, err := writeCSV(headers, rows)
		return data, ext, err
	}
}

func buildLeaveLookup(leaves []domain.LeaveRequest) map[string]map[string]bool {
	if len(leaves) == 0 {
		return nil
	}
	lookup := make(map[string]map[string]bool)
	for _, l := range leaves {
		uid := l.UserID.Hex()
		if lookup[uid] == nil {
			lookup[uid] = make(map[string]bool)
		}
		dateStr := l.Date
		if dateStr != "" {
			lookup[uid][dateStr] = true
		}
	}
	return lookup
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func ExportSalesforceCSV(cohort int, startDate, endDate string, recordRepo domain.AttendanceRepository, userService *userService.Service) ([]byte, error) {
	req := ExportRequest{
		Cohort:    cohort,
		StartDate: startDate,
		EndDate:   endDate,
		Format:    ExportFormatCSV,
		Structure: ExportStructureDaily,
		SplitAMPM: false,
	}
	data, _, err := Export(req, recordRepo, userService)
	return data, err
}

func (s *ExportService) ExportSalesforceCSV(cohort int, startDate, endDate string) ([]byte, error) {
	return ExportSalesforceCSV(cohort, startDate, endDate, s.recordRepo, s.userService)
}

func (s *ExportService) Export(req ExportRequest) ([]byte, string, error) {
	req.LeaveData = s.fillLeaveData(req)
	return Export(req, s.recordRepo, s.userService)
}

func (s *ExportService) fillLeaveData(req ExportRequest) []domain.LeaveRequest {
	return req.LeaveData
}
