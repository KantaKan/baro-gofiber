package main

import (
	"gofiber-baro/internal/service/report"
	"gofiber-baro/internal/service/slack"
)

// reportJob adapts the Slack settings and the report service to the Friday job.
type reportJob struct {
	slack *slack.Service
	rep   *report.Service
}

func (r reportJob) AutoSettings() (bool, int, error)             { return r.slack.ReportAuto() }
func (r reportJob) IsSent(cohort int, week string) (bool, error) { return r.rep.IsSent(cohort, week) }
func (r reportJob) Send(cohort int, week string) (report.SendResult, error) {
	return r.rep.Send(cohort, week)
}
