package handler

import (
	"testing"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/staff"
)

func TestTAMayApprove(t *testing.T) {
	if !taMayApprove(staff.TA, "approved", 6) || taMayApprove(staff.TA, "approved", 7) {
		t.Error("TA limit is 6 of 8")
	}
	if !taMayApprove(staff.TA, "rejected", 8) || !taMayApprove(staff.Admin, "approved", 8) {
		t.Error("only TA approvals are limited")
	}
}

func TestTAMayAnnounce(t *testing.T) {
	ta := staff.Ctx{Role: staff.TA, Cohorts: []int{13}, Groups: []string{"B", "C"}}
	in := &domain.User{CohortNumber: 13, GenmateGroup: "B"}
	out := &domain.User{CohortNumber: 13, GenmateGroup: "D"}
	other := &domain.User{CohortNumber: 14, GenmateGroup: "B"}
	if !taMayAnnounce(ta, []*domain.User{in}) {
		t.Error("own group allowed")
	}
	if taMayAnnounce(ta, []*domain.User{in, out}) || taMayAnnounce(ta, []*domain.User{other}) || taMayAnnounce(ta, nil) || taMayAnnounce(ta, []*domain.User{nil}) {
		t.Error("outside own groups, other cohort, everyone or unknown must be refused")
	}
	if !taMayAnnounce(staff.Ctx{Role: staff.Admin}, nil) {
		t.Error("admins aren't limited")
	}
}
