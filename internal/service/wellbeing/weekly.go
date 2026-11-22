package wellbeing

import (
	"sort"
	"time"

	"gofiber-baro/internal/domain"
)

// RoughLearner is someone who had at least one overwhelmed or panic night in a week.
type RoughLearner struct {
	Learner     LearnerRef `json:"learner"`
	Nights      [5]string  `json:"nights"` // Mon–Fri zone ids ("" = nothing written)
	Panic       int        `json:"panic"`
	Overwhelmed int        `json:"overwhelmed"`
	Again       bool       `json:"again"` // also rough the week before
}

// WeekRough summarises one Mon–Fri week of reflections for a cohort.
type WeekRough struct {
	Week     string         `json:"week"` // Monday, YYYY-MM-DD
	Sent     int            `json:"sent"`
	Possible int            `json:"possible"`
	Zones    map[string]int `json:"zones"`
	Learners []RoughLearner `json:"learners"`
}

func weekdays(monday string) [5]string {
	var out [5]string
	m, _ := time.Parse("2006-01-02", monday)
	for i := range out {
		out[i] = m.AddDate(0, 0, i).Format("2006-01-02")
	}
	return out
}

func rough(zone string) bool { return zone == "panic" || zone == "stretch-overwhelmed" }

// WeeklyRough builds one summary per Monday (newest first, as given). Days after today and holidays
// don't count as missed nights; dropped-out learners are left out.
func WeeklyRough(users []domain.User, mondays []string, today string, holidays map[string]bool) []WeekRough {
	active := Active(users)
	nights := make([]map[string]string, len(active))
	for i, u := range active {
		nights[i] = map[string]string{}
		for _, n := range Nights(u.Reflections) {
			nights[i][n.Day] = n.Zone
		}
	}
	roughIn := func(i int, monday string) bool {
		for _, d := range weekdays(monday) {
			if rough(nights[i][d]) {
				return true
			}
		}
		return false
	}
	out := make([]WeekRough, 0, len(mondays))
	for _, monday := range mondays {
		days := weekdays(monday)
		prev := func() string {
			m, _ := time.Parse("2006-01-02", monday)
			return m.AddDate(0, 0, -7).Format("2006-01-02")
		}()
		w := WeekRough{Week: monday, Zones: map[string]int{"comfort": 0, "stretch-enjoying": 0, "stretch-overwhelmed": 0, "panic": 0}, Learners: []RoughLearner{}}
		for i, u := range active {
			var rl RoughLearner
			for d, day := range days {
				counts := day <= today && !holidays[day]
				if counts {
					w.Possible++
				}
				zone, wrote := nights[i][day]
				if !wrote {
					continue
				}
				if counts {
					w.Sent++
				}
				if _, ok := w.Zones[zone]; ok {
					w.Zones[zone]++
				}
				rl.Nights[d] = zone
				switch zone {
				case "panic":
					rl.Panic++
				case "stretch-overwhelmed":
					rl.Overwhelmed++
				}
			}
			if rl.Panic+rl.Overwhelmed > 0 {
				rl.Learner, rl.Again = Ref(u), roughIn(i, prev)
				w.Learners = append(w.Learners, rl)
			}
		}
		sort.SliceStable(w.Learners, func(a, b int) bool {
			x, y := w.Learners[a], w.Learners[b]
			if x.Panic != y.Panic {
				return x.Panic > y.Panic
			}
			if x.Overwhelmed != y.Overwhelmed {
				return x.Overwhelmed > y.Overwhelmed
			}
			return x.Learner.FirstName < y.Learner.FirstName
		})
		out = append(out, w)
	}
	return out
}
