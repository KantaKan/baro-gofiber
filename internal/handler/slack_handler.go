package handler

import (
	"gofiber-baro/internal/service/slack"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type slackService interface {
	Status() (slack.Status, error)
	Connect(token string) error
	Disconnect() error
	SaveSettings(slack.Settings) error
	Channels() ([]slack.Channel, error)
	Test(which string) error
	Match(cohort int) (slack.MatchResult, error)
	Log(limit int) ([]slack.LogEntry, error)
	Ping(ids []primitive.ObjectID) (slack.PingResult, error)
}

// SlackHandler: Settings → Slack (Owner only, via StaffGuard) and the Ping nudge.
type SlackHandler struct{ svc slackService }

func NewSlackHandler(s slackService) *SlackHandler { return &SlackHandler{svc: s} }

// Status: connection, channels and toggles (never the token), plus the app manifest to paste into Slack.
// @Summary  Slack settings  @Tags admin  @Security BearerAuth  @Router /admin/settings/slack [get]
func (h *SlackHandler) Status(c *fiber.Ctx) error {
	st, err := h.svc.Status()
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Couldn't load Slack settings")
	}
	return utils.SendResponse(c, fiber.StatusOK, "ok", fiber.Map{
		"connected": st.Connected, "team": st.Team, "bot": st.Bot,
		"alerts": st.Alerts, "report": st.Report, "announce": st.Announce,
		"alert_new_leave": st.AlertNewLeave, "quiet_from": st.QuietFrom, "quiet_to": st.QuietTo,
		"manifest": slack.Manifest(),
	})
}

// Connect verifies and stores a bot token.
// @Summary  Connect Slack  @Tags admin  @Security BearerAuth  @Router /admin/settings/slack/connect [post]
func (h *SlackHandler) Connect(c *fiber.Ctx) error {
	var b struct {
		Token string `json:"token"`
	}
	if err := c.BodyParser(&b); err != nil || b.Token == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Paste the bot token (it starts with xoxb-)")
	}
	if err := h.svc.Connect(b.Token); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return h.Status(c)
}

// @Summary  Disconnect Slack  @Tags admin  @Security BearerAuth  @Router /admin/settings/slack [delete]
func (h *SlackHandler) Disconnect(c *fiber.Ctx) error {
	if err := h.svc.Disconnect(); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Couldn't disconnect")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Disconnected", nil)
}

// @Summary  Save channels, alerts and quiet hours  @Tags admin  @Security BearerAuth  @Router /admin/settings/slack/settings [put]
func (h *SlackHandler) SaveSettings(c *fiber.Ctx) error {
	var b struct {
		Alerts        string `json:"alerts"`
		Report        string `json:"report"`
		Announce      string `json:"announce"`
		AlertNewLeave bool   `json:"alert_new_leave"`
		QuietFrom     string `json:"quiet_from"`
		QuietTo       string `json:"quiet_to"`
	}
	if err := c.BodyParser(&b); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	if err := h.svc.SaveSettings(slack.Settings{Alerts: b.Alerts, Report: b.Report, Announce: b.Announce, AlertNewLeave: b.AlertNewLeave, QuietFrom: b.QuietFrom, QuietTo: b.QuietTo}); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Saved", nil)
}

// @Summary  Channels the bot can see  @Tags admin  @Security BearerAuth  @Router /admin/settings/slack/channels [get]
func (h *SlackHandler) Channels(c *fiber.Ctx) error {
	chs, err := h.svc.Channels()
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "ok", chs)
}

// @Summary  Send a test message  @Tags admin  @Security BearerAuth  @Router /admin/settings/slack/test [post]
func (h *SlackHandler) Test(c *fiber.Ctx) error {
	var b struct {
		Channel string `json:"channel"`
	}
	if err := c.BodyParser(&b); err != nil || (b.Channel != "alerts" && b.Channel != "report" && b.Channel != "announce") {
		return utils.SendError(c, fiber.StatusBadRequest, "channel must be alerts, report or announce")
	}
	if err := h.svc.Test(b.Channel); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "Sent", nil)
}

// @Summary  Match learners to Slack by email  @Tags admin  @Security BearerAuth  @Router /admin/settings/slack/match [post]
func (h *SlackHandler) Match(c *fiber.Ctx) error {
	var b struct {
		Cohort int `json:"cohort"`
	}
	if err := c.BodyParser(&b); err != nil || b.Cohort <= 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "cohort is required")
	}
	res, err := h.svc.Match(b.Cohort)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "ok", res)
}

// @Summary  Recent Slack sends  @Tags admin  @Security BearerAuth  @Router /admin/settings/slack/log [get]
func (h *SlackHandler) Log(c *fiber.Ctx) error {
	l, err := h.svc.Log(20)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Couldn't load the log")
	}
	return utils.SendResponse(c, fiber.StatusOK, "ok", l)
}

// Ping DMs a check-in reminder to the chosen learners.
// @Summary  Ping learners who haven't checked in  @Tags admin  @Security BearerAuth  @Router /admin/nudges/ping [post]
func (h *SlackHandler) Ping(c *fiber.Ctx) error {
	var b struct {
		UserIDs []string `json:"user_ids"`
	}
	if err := c.BodyParser(&b); err != nil || len(b.UserIDs) == 0 || len(b.UserIDs) > 200 {
		return utils.SendError(c, fiber.StatusBadRequest, "Choose between 1 and 200 learners")
	}
	ids := make([]primitive.ObjectID, 0, len(b.UserIDs))
	for _, s := range b.UserIDs {
		id, err := primitive.ObjectIDFromHex(s)
		if err != nil {
			return utils.SendError(c, fiber.StatusBadRequest, "Invalid learner id")
		}
		ids = append(ids, id)
	}
	res, err := h.svc.Ping(ids)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.SendResponse(c, fiber.StatusOK, "ok", res)
}
