package startupstory

import (
	"fmt"
	"hash/fnv"
	"time"
)

const MaxRankedAttempts = 3

func bangkokLocation() *time.Location {
	if loc, err := time.LoadLocation("Asia/Bangkok"); err == nil {
		return loc
	}
	return time.FixedZone("ICT", 7*3600)
}

func weekKeyFor(t time.Time) string {
	y, w := t.In(bangkokLocation()).ISOWeek()
	return fmt.Sprintf("%04d-W%02d", y, w)
}

func WeekSeed(weekKey string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("baro-startup-story|" + weekKey))
	return h.Sum64()
}
