package startupstory

import (
	"fmt"

	"gofiber-baro/internal/domain"
)

const (
	SetbackFansKeep   = 0.8
	SetbackBurnout    = 15
	LoanCash          = 5000
	LoanDebt          = 10000
	LoanRepayPercent  = 25
	FounderBreakReset = 40
	FameNewStage      = 15
	FameBossBeaten    = 10
)

func isCompany(run *domain.StartupRun) bool {
	return run.Mode != domain.StartupModeRanked
}

func bossSetback(run *domain.StartupRun) {
	run.Fans = int(float64(run.Fans) * SetbackFansKeep)
	for i := range run.Staff {
		run.Staff[i].Burnout = min(BurnoutQuitAt-1, run.Staff[i].Burnout+SetbackBurnout)
	}
	run.ProjectIndex = max(0, run.ProjectIndex-3)
	addLog(run, "The demo flopped. We regroup: two more projects, then a rematch.")
}

func emergencyLoan(run *domain.StartupRun) {
	run.Money = LoanCash
	run.Debt += LoanDebt
	addLog(run, fmt.Sprintf("The bank called. They were not impressed. Emergency loan: ฿%d to keep going, ฿%d to pay back.", LoanCash, LoanDebt))
}

func repayDebt(run *domain.StartupRun, earned int) {
	if run.Debt <= 0 || earned <= 0 {
		return
	}
	pay := min(run.Debt, earned*LoanRepayPercent/100)
	run.Money -= pay
	run.Debt -= pay
	if run.Debt == 0 {
		addLog(run, "Loan paid off. The bank sends a fruit basket.")
	}
}

func founderBreak(run *domain.StartupRun, d *domain.StartupDev) {
	d.Burnout = FounderBreakReset
	addLog(run, fmt.Sprintf("%s took a long weekend in Hua Hin and came back slightly less tired.", d.Name))
}

func noDebt(run *domain.StartupRun) error {
	if run.Debt > 0 {
		return domain.ErrStartupInDebt
	}
	return nil
}
