package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestToSafeHidesPrivateData(t *testing.T) {
	s := "variant20"
	u := User{
		Email: "a@b.c", Password: "hash", AttendanceStatus: "dropout",
		Reflections:  []Reflection{{}},
		CheckinStyle: "rocket", CheckinRandom: true,
		Avatar: &Avatar{Style: "notionists", Seed: "x", Bg: "d1fae5", Options: map[string]map[string]*string{"notionists": {"hair": &s}}},
		Skills: []string{"React"},
	}
	b, err := json.Marshal(u.ToSafe())
	if err != nil {
		t.Fatal(err)
	}
	js := string(b)
	for _, must := range []string{`"avatar"`, `"skills"`} {
		if !strings.Contains(js, must) {
			t.Errorf("missing %s in %s", must, js)
		}
	}
	for _, never := range []string{"attendance_status", "reflections", "checkin_style", "checkin_random", "email", "password"} {
		if strings.Contains(js, never) {
			t.Errorf("leaked %q in %s", never, js)
		}
	}
}
