package utils

import (
	"time"
)

var thailandLocation *time.Location

func init() {
	thailandLocation, _ = time.LoadLocation("Asia/Bangkok")
}

func GetThailandTime() time.Time {
	return time.Now().In(thailandLocation)
}

func GetThailandDate() string {
	return GetThailandTime().Format("2006-01-02")
}

func ParseDate(dateStr string) (time.Time, error) {
	return time.Parse("2006-01-02", dateStr)
}

func FormatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

func StartOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func EndOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, t.Location())
}

func WeekToDate(year, week int) (time.Time, time.Time) {
	t := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)

	weekday := t.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	t = t.AddDate(0, 0, int(time.Monday-weekday))

	t = t.AddDate(0, 0, (week-1)*7)

	startDate := t
	endDate := t.AddDate(0, 0, 6)

	return startDate, endDate
}
