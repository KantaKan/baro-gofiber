package cup

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Store persists awards and wins.
type Store interface {
	Insert(ctx context.Context, a *domain.CupAward) error
	InsertOnce(ctx context.Context, a *domain.CupAward) (bool, error) // false when Key exists
	List(ctx context.Context, cohort int, week string) ([]domain.CupAward, error)
	Delete(ctx context.Context, id primitive.ObjectID) error
	InsertWinOnce(ctx context.Context, w *domain.CupWin) (bool, error)
	Wins(ctx context.Context, cohort int) ([]domain.CupWin, error)
	Groups(ctx context.Context, cohort int) (map[string][]primitive.ObjectID, error) // active learners per genmate group
}

// SettingsGetter gives the cohort start date for week numbers.
type SettingsGetter interface {
	Get(cohort int) (domain.CohortSettings, error)
}

type Service struct {
	store    Store
	settings SettingsGetter
}

func NewService(store Store, settings SettingsGetter) *Service {
	return &Service{store: store, settings: settings}
}

var (
	ErrInvalidAward = errors.New("invalid award")
	awardable       = map[string]bool{CatCoding: true, CatSoft: true, CatHelping: true}
)

func ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

// Award adds TA points to a group for the current week.
func (s *Service) Award(adminID string, cohort int, group, category string, points int, reason string, now time.Time) (*domain.CupAward, error) {
	by, err := primitive.ObjectIDFromHex(adminID)
	if err != nil {
		return nil, fmt.Errorf("%w: admin", ErrInvalidAward)
	}
	group, reason = strings.TrimSpace(group), strings.TrimSpace(reason)
	if !awardable[category] {
		return nil, fmt.Errorf("%w: category must be coding, soft or helping", ErrInvalidAward)
	}
	if points < 1 || points > 100 {
		return nil, fmt.Errorf("%w: points must be 1-100", ErrInvalidAward)
	}
	if reason == "" || len([]rune(reason)) > 140 {
		return nil, fmt.Errorf("%w: reason must be 1-140 characters", ErrInvalidAward)
	}
	c, cancel := ctx()
	defer cancel()
	groups, err := s.store.Groups(c, cohort)
	if err != nil {
		return nil, err
	}
	if _, ok := groups[group]; !ok {
		return nil, fmt.Errorf("%w: unknown group %q", ErrInvalidAward, group)
	}
	a := &domain.CupAward{Cohort: cohort, Group: group, Category: category, Points: points, Reason: reason,
		Day: now.In(bangkok).Format(dateFmt), Week: WeekStart(now), AwardedBy: by, CreatedAt: now}
	if err := s.store.Insert(c, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *Service) Delete(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("%w: id", ErrInvalidAward)
	}
	c, cancel := ctx()
	defer cancel()
	return s.store.Delete(c, oid)
}

func (s *Service) weekNo(cohort int, week string) int {
	start := ""
	if s.settings != nil {
		if st, err := s.settings.Get(cohort); err == nil {
			start = st.StartDate
		}
	}
	return WeekNo(start, week)
}

func groupNames(g map[string][]primitive.ObjectID) []string {
	out := make([]string, 0, len(g))
	for k := range g {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Standings for a cohort's week.
func (s *Service) Standings(cohort int, week string) ([]GroupScore, []domain.CupAward, error) {
	c, cancel := ctx()
	defer cancel()
	groups, err := s.store.Groups(c, cohort)
	if err != nil {
		return nil, nil, err
	}
	awards, err := s.store.List(c, cohort, week)
	if err != nil {
		return nil, nil, err
	}
	return Standings(awards, groupNames(groups), week), awards, nil
}

type Medal struct {
	Week   string `json:"week"`
	WeekNo int    `json:"week_no"`
}

type FameRow struct {
	WeekNo int    `json:"week_no"`
	Group  string `json:"group"`
	Points int    `json:"points"`
}

// MyCup is everything the learner's Cup cards need.
type MyCup struct {
	Week          string            `json:"week"`
	WeekNo        int               `json:"week_no"`
	EndsAt        time.Time         `json:"ends_at"`
	Standings     []GroupScore      `json:"standings"`
	MyGroup       string            `json:"my_group"`
	MyAwards      []domain.CupAward `json:"my_awards"`
	ChampionGroup string            `json:"champion_group"`
	MyMedals      []Medal           `json:"my_medals"`
	WallOfFame    []FameRow         `json:"wall_of_fame"`
}

// IsChampion: the learner was in last week's winning group (frozen members).
func (s *Service) IsChampion(u domain.User, now time.Time) bool {
	c, cancel := ctx()
	defer cancel()
	wins, err := s.store.Wins(c, u.CohortNumber)
	if err != nil {
		return false
	}
	prev := WeekStart(WeekEnd(WeekStart(now)).AddDate(0, 0, -7))
	for _, w := range wins {
		if w.Week == prev {
			for _, m := range w.Members {
				if m == u.ID {
					return true
				}
			}
		}
	}
	return false
}

func (s *Service) Me(u domain.User, now time.Time) (*MyCup, error) {
	week := WeekStart(now)
	standings, awards, err := s.Standings(u.CohortNumber, week)
	if err != nil {
		return nil, err
	}
	c, cancel := ctx()
	defer cancel()
	wins, err := s.store.Wins(c, u.CohortNumber)
	if err != nil {
		return nil, err
	}
	sort.Slice(wins, func(i, j int) bool { return wins[i].Week > wins[j].Week })
	out := &MyCup{Week: week, WeekNo: s.weekNo(u.CohortNumber, week), EndsAt: WeekEnd(week), Standings: standings,
		MyGroup: u.GenmateGroup, MyAwards: []domain.CupAward{}, MyMedals: []Medal{}, WallOfFame: []FameRow{}}
	prev := WeekStart(WeekEnd(week).AddDate(0, 0, -7))
	for _, w := range wins {
		if w.Week == prev {
			out.ChampionGroup = w.Group
		}
		out.WallOfFame = append(out.WallOfFame, FameRow{WeekNo: w.WeekNo, Group: w.Group, Points: w.Points})
		for _, m := range w.Members {
			if m == u.ID {
				out.MyMedals = append(out.MyMedals, Medal{Week: w.Week, WeekNo: w.WeekNo})
				break
			}
		}
	}
	if u.GenmateGroup != "" {
		sort.SliceStable(awards, func(i, j int) bool { return awards[i].CreatedAt.After(awards[j].CreatedAt) })
		for _, a := range awards {
			if a.Group == u.GenmateGroup && len(out.MyAwards) < 6 {
				out.MyAwards = append(out.MyAwards, a)
			}
		}
	}
	return out, nil
}
