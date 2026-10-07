package startupstory

import "testing"

func TestInfraTeachesScaling(t *testing.T) {
	without, with, reachedAct3 := 0, 0, 0
	for seed := uint64(1); seed <= 200; seed++ {
		_, _, _, _, act := playBotInfra(t, seed, true, false)
		if act > 0 && act <= 2 {
			without++
		}
		_, deathAct, _, _, act := playBotInfra(t, seed, true, true)
		if deathAct >= 3 {
			reachedAct3++
			if act > 0 && act <= 3 {
				with++
			}
		}
	}
	t.Logf("no infra: %d/200 overloaded by act 2; sensible infra: %d/%d overloaded in acts 1-3", without, with, reachedAct3)
	if without < 150 {
		t.Errorf("a team that never scales should overload by act 2 in most runs, got %d/200", without)
	}
	if with*100 > reachedAct3*15 {
		t.Errorf("a team that scales sensibly should rarely overload in acts 1-3 (only DB spikes, which cache/replicas fix in 25b), got %d/%d", with, reachedAct3)
	}
}
