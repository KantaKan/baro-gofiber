package attendance

import "gofiber-baro/internal/domain"

// The rule types live in domain (so CohortSettings can store them); these aliases
// keep the attendance package's vocabulary short.
type (
	Clock         = domain.Clock
	SessionWindow = domain.SessionWindow
	CheckinRules  = domain.CheckinRules
	Phase         = domain.Phase
)

const (
	PhaseNotOpen = domain.PhaseNotOpen
	PhaseOnTime  = domain.PhaseOnTime
	PhaseLate    = domain.PhaseLate
	PhaseClosed  = domain.PhaseClosed
)

func DefaultRules() CheckinRules { return domain.DefaultRules() }

type statusPair struct{ m, a domain.AttendanceStatus }

// dayTable is the "TH-Status Criteria" sheet: morning + afternoon -> day status.
var dayTable = func() map[statusPair]domain.AttendanceStatus {
	P, A, AE, H, L, LE, N := domain.StatusPresent, domain.StatusAbsent, domain.StatusAbsentExcused, domain.StatusHoliday, domain.StatusLate, domain.StatusLateExcused, domain.StatusNoClass
	return map[statusPair]domain.AttendanceStatus{
		{P, P}: P, {P, A}: A, {P, AE}: AE, {P, H}: H, {P, L}: L, {P, LE}: LE, {P, N}: P,
		{A, P}: A, {A, A}: A, {A, AE}: A, {A, H}: H, {A, L}: A, {A, LE}: A, {A, N}: A,
		{AE, P}: AE, {AE, A}: AE, {AE, AE}: AE, {AE, H}: H, {AE, L}: AE, {AE, LE}: AE, {AE, N}: AE,
		{L, P}: L, {L, A}: A, {L, AE}: AE, {L, H}: H, {L, L}: L, {L, LE}: L, {L, N}: L,
		{LE, P}: LE, {LE, A}: A, {LE, AE}: AE, {LE, H}: H, {LE, L}: L, {LE, LE}: LE, {LE, N}: LE,
		{N, P}: P, {N, A}: A, {N, AE}: AE, {N, H}: H, {N, L}: L, {N, LE}: LE, {N, N}: N,
		{H, P}: H, {H, A}: H, {H, AE}: H, {H, H}: H, {H, L}: H, {H, LE}: H, {H, N}: H,
	}
}()

// CombineDayStatus turns a morning and afternoon status into the day's status.
// Dropout/dismissed on either side win; a missing session counts as no_class.
func CombineDayStatus(morning, afternoon domain.AttendanceStatus) domain.AttendanceStatus {
	for _, s := range []domain.AttendanceStatus{domain.StatusDropout, domain.StatusDismissed} {
		if morning == s || afternoon == s {
			return s
		}
	}
	if morning == "" {
		morning = domain.StatusNoClass
	}
	if afternoon == "" {
		afternoon = domain.StatusNoClass
	}
	if d, ok := dayTable[statusPair{morning, afternoon}]; ok {
		return d
	}
	return domain.StatusNoClass
}
