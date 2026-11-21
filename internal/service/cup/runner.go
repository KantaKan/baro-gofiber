package cup

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/giftbox"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Records is the slice of the attendance repository the runner reads.
type Records interface {
	FindRecords(ctx interface{}, filter domain.AttendanceRecordFilter, opts interface{}) ([]domain.AttendanceRecord, error)
	ActiveCohorts(ctx interface{}, since string) ([]int, error)
}

// Granter is main's gift-box audience grant.
type Granter interface {
	GrantAudience(ctx context.Context, cohort int, team, adminID, minimumRarity, message, idempotencyKey, rewardPool string) (*giftbox.CohortGrantResult, error)
}

// Runner adds the daily showing-up bonus and closes finished weeks. Every write is
// keyed, so ticks, restarts and several instances never double-award.
type Runner struct {
	svc      *Service
	store    Store
	records  Records
	settings SettingsGetter
	gifts    Granter
	classDay func(time.Time) (bool, error)

	mu   sync.Mutex
	done map[string]bool
}

func NewRunner(svc *Service, store Store, records Records, settings SettingsGetter, gifts Granter, classDay func(time.Time) (bool, error)) *Runner {
	return &Runner{svc: svc, store: store, records: records, settings: settings, gifts: gifts, classDay: classDay, done: map[string]bool{}}
}

func (r *Runner) isDone(k string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.done[k]
}

func (r *Runner) markDone(k string) {
	r.mu.Lock()
	r.done[k] = true
	r.mu.Unlock()
}

// RunDue does whatever is due at now; errors are logged per cohort and retried next tick.
func (r *Runner) RunDue(now time.Time) error {
	now = now.In(bangkok)
	c, cancel := ctx()
	cohorts, err := r.records.ActiveCohorts(c, now.AddDate(0, 0, -14).Format(dateFmt))
	cancel()
	if err != nil {
		return err
	}
	var all error
	for _, cohort := range cohorts {
		if err := r.showingUp(cohort, now); err != nil {
			log.Printf("cup: cohort %d showing-up: %v", cohort, err)
			all = errors.Join(all, err)
		}
		for _, week := range []string{WeekStart(now.AddDate(0, 0, -7)), WeekStart(now)} {
			if err := r.closeWeek(cohort, week, now); err != nil {
				log.Printf("cup: cohort %d week %s: %v", cohort, week, err)
				all = errors.Join(all, err)
			}
		}
	}
	return all
}

// showingUp checks today (once the afternoon closes) and, every 10 minutes, this
// week's earlier days, so an excuse approved later still earns the bonus before Friday's close.
func (r *Runner) showingUp(cohort int, now time.Time) error {
	week := WeekStart(now)
	if !now.Before(WeekEnd(week)) {
		return nil
	}
	st, err := r.settings.Get(cohort)
	if err != nil {
		return err
	}
	mon, _ := parseDay(week)
	var all error
	for d := mon; !d.After(now); d = d.AddDate(0, 0, 1) {
		date := d.Format(dateFmt)
		today := date == now.Format(dateFmt)
		if today && !now.After(st.Rules.Afternoon.AbsentAfter.On(now)) {
			continue
		}
		if !today && now.Minute()%10 != 0 {
			continue
		}
		if err := r.showingUpDay(cohort, date, d.Add(12*time.Hour), week); err != nil {
			all = errors.Join(all, err)
		}
	}
	return all
}

func (r *Runner) showingUpDay(cohort int, date string, noon time.Time, week string) error {
	if r.isDone(fmt.Sprintf("show|%d|%s", cohort, date)) {
		return nil
	}
	if ok, err := r.classDay(noon); err != nil || !ok {
		if err == nil {
			r.markDone(fmt.Sprintf("show|%d|%s", cohort, date))
		}
		return err
	}
	c, cancel := ctx()
	defer cancel()
	groups, err := r.store.Groups(c, cohort)
	if err != nil {
		return err
	}
	recs, err := r.records.FindRecords(c, domain.AttendanceRecordFilter{Cohort: cohort, Date: date, NotDeleted: true}, nil)
	if err != nil {
		return err
	}
	statuses := map[primitive.ObjectID][]domain.AttendanceStatus{}
	for _, rec := range recs {
		statuses[rec.UserID] = append(statuses[rec.UserID], rec.Status)
	}
	pending := false
	now := noon
	for group, members := range groups {
		key := fmt.Sprintf("showup:%d:%s:%s", cohort, group, date)
		if r.isDone(key) {
			continue
		}
		complete := true
		for _, m := range members {
			if len(statuses[m]) < 2 {
				complete = false // the session closer hasn't written everyone yet
			}
		}
		if !complete {
			pending = true
			continue
		}
		if !WholeGroupOnTime(members, statuses) {
			pending = true // an approved excuse may still change a late/absent this week
			continue
		}
		a := &domain.CupAward{Cohort: cohort, Group: group, Category: CatShowingUp, Points: ShowingUpPoints,
			Reason: "Whole group on time", Day: date, Week: week, Key: key, CreatedAt: now}
		if _, err := r.store.InsertOnce(c, a); err != nil {
			return err
		}
		r.markDone(key)
	}
	if !pending {
		r.markDone(fmt.Sprintf("show|%d|%s", cohort, date))
	}
	return nil
}

func (r *Runner) closeWeek(cohort int, week string, now time.Time) error {
	key := fmt.Sprintf("week|%d|%s", cohort, week)
	if r.isDone(key) || now.Before(WeekEnd(week)) {
		return nil
	}
	c, cancel := ctx()
	defer cancel()
	wins, err := r.store.Wins(c, cohort)
	if err != nil {
		return err
	}
	for _, w := range wins {
		if w.Week == week {
			r.markDone(key)
			return nil
		}
	}
	standings, _, err := r.svc.Standings(cohort, week)
	if err != nil {
		return err
	}
	group, ok := Winner(standings)
	if !ok {
		r.markDone(key)
		return nil
	}
	groups, err := r.store.Groups(c, cohort)
	if err != nil {
		return err
	}
	no := r.svc.weekNo(cohort, week)
	res, err := r.gifts.GrantAudience(c, cohort, group, primitive.NilObjectID.Hex(), "Rare",
		fmt.Sprintf("GenMate Cup · Week %d champions", no), fmt.Sprintf("cup:%d:%s", cohort, week), "character-box")
	if err != nil {
		return err
	}
	if len(res.Failures) > 0 {
		return fmt.Errorf("gift failed for %d learners", len(res.Failures))
	}
	// Record the win only after every gift landed, so a failure is retried next tick.
	if _, err := r.store.InsertWinOnce(c, &domain.CupWin{Cohort: cohort, Week: week, WeekNo: no, Group: group,
		Points: standings[0].Total, Members: groups[group], CreatedAt: now}); err != nil {
		return err
	}
	r.markDone(key)
	return nil
}
