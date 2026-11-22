package handler

import (
	"context"
	"strings"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CareStore keeps TA notes, support-plan items and acknowledgements.
type CareStore interface {
	AddNote(ctx context.Context, n *domain.AdminNote) error
	Notes(ctx context.Context, userID primitive.ObjectID) ([]domain.AdminNote, error)
	Note(ctx context.Context, id primitive.ObjectID) (*domain.AdminNote, error)
	DeleteNote(ctx context.Context, id primitive.ObjectID) error
	AddItem(ctx context.Context, it *domain.SupportItem) error
	Items(ctx context.Context, userID primitive.ObjectID) ([]domain.SupportItem, error)
	SetItemDone(ctx context.Context, id primitive.ObjectID, done bool) error
	DeleteItem(ctx context.Context, id primitive.ObjectID) error
	Ack(ctx context.Context, a *domain.SupportAck) error
	Acks(ctx context.Context, userID primitive.ObjectID, now time.Time) ([]domain.SupportAck, error)
}

type careUserLookup interface {
	GetUserByID(id string) (*domain.User, error)
}

// LearnerCareHandler serves private notes and support plans for admins.
type LearnerCareHandler struct {
	store CareStore
	users careUserLookup
}

func NewLearnerCareHandler(store CareStore, users careUserLookup) *LearnerCareHandler {
	return &LearnerCareHandler{store: store, users: users}
}

func careCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

// learner resolves :id; when ok is false the error response has already been written.
func (h *LearnerCareHandler) learner(c *fiber.Ctx) (id primitive.ObjectID, ok bool, err error) {
	id, e := primitive.ObjectIDFromHex(c.Params("id"))
	if e != nil {
		return id, false, utils.SendError(c, fiber.StatusBadRequest, "Invalid learner ID")
	}
	if u, e := h.users.GetUserByID(id.Hex()); e != nil || u == nil {
		return id, false, utils.SendError(c, fiber.StatusNotFound, "Learner not found")
	}
	return id, true, nil
}

func (h *LearnerCareHandler) me(c *fiber.Ctx) (primitive.ObjectID, string) {
	s, _ := c.Locals("userID").(string)
	id, _ := primitive.ObjectIDFromHex(s)
	name := "Admin"
	if u, err := h.users.GetUserByID(s); err == nil && u != nil && u.FirstName != "" {
		name = u.FirstName
	}
	return id, name
}

func text(c *fiber.Ctx, max int) (string, bool) {
	var b struct {
		Text string `json:"text"`
	}
	if err := c.BodyParser(&b); err != nil {
		return "", false
	}
	t := strings.TrimSpace(b.Text)
	return t, t != "" && len([]rune(t)) <= max
}

// Notes lists a learner's private TA notes (newest first).
// @Summary  List TA notes  @Tags admin  @Security BearerAuth  @Router /admin/learners/{id}/notes [get]
func (h *LearnerCareHandler) Notes(c *fiber.Ctx) error {
	id, ok, err := h.learner(c)
	if !ok {
		return err
	}
	ctx, cancel := careCtx()
	defer cancel()
	n, e := h.store.Notes(ctx, id)
	if e != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading notes")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Notes", n)
}

// AddNote adds a private TA note.
// @Summary  Add TA note  @Tags admin  @Security BearerAuth  @Router /admin/learners/{id}/notes [post]
func (h *LearnerCareHandler) AddNote(c *fiber.Ctx) error {
	id, ok, err := h.learner(c)
	if !ok {
		return err
	}
	t, valid := text(c, 2000)
	if !valid {
		return utils.SendError(c, fiber.StatusBadRequest, "Note must be 1–2000 characters")
	}
	me, name := h.me(c)
	n := &domain.AdminNote{UserID: id, AuthorID: me, AuthorName: name, Text: t, CreatedAt: time.Now()}
	ctx, cancel := careCtx()
	defer cancel()
	if e := h.store.AddNote(ctx, n); e != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error saving note")
	}
	return utils.SendResponse(c, fiber.StatusCreated, "Note saved", n)
}

// DeleteNote removes your own note.
// @Summary  Delete TA note  @Tags admin  @Security BearerAuth  @Router /admin/notes/{noteId} [delete]
func (h *LearnerCareHandler) DeleteNote(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("noteId"))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid note ID")
	}
	ctx, cancel := careCtx()
	defer cancel()
	n, e := h.store.Note(ctx, id)
	if e != nil {
		return utils.SendError(c, fiber.StatusNotFound, "Note not found")
	}
	if me, _ := h.me(c); n.AuthorID != me {
		return utils.SendError(c, fiber.StatusForbidden, "You can only delete your own notes")
	}
	if e := h.store.DeleteNote(ctx, id); e != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error deleting note")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Note deleted", nil)
}

// Support lists a learner's support-plan items.
// @Summary  List support plan  @Tags admin  @Security BearerAuth  @Router /admin/learners/{id}/support [get]
func (h *LearnerCareHandler) Support(c *fiber.Ctx) error {
	id, ok, err := h.learner(c)
	if !ok {
		return err
	}
	ctx, cancel := careCtx()
	defer cancel()
	it, e := h.store.Items(ctx, id)
	if e != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading support plan")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Support plan", it)
}

// AddSupport adds a support-plan step.
// @Summary  Add support step  @Tags admin  @Security BearerAuth  @Router /admin/learners/{id}/support [post]
func (h *LearnerCareHandler) AddSupport(c *fiber.Ctx) error {
	id, ok, err := h.learner(c)
	if !ok {
		return err
	}
	t, valid := text(c, 200)
	if !valid {
		return utils.SendError(c, fiber.StatusBadRequest, "Step must be 1–200 characters")
	}
	_, name := h.me(c)
	it := &domain.SupportItem{UserID: id, Text: t, AuthorName: name, CreatedAt: time.Now()}
	ctx, cancel := careCtx()
	defer cancel()
	if e := h.store.AddItem(ctx, it); e != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error saving step")
	}
	return utils.SendResponse(c, fiber.StatusCreated, "Step added", it)
}

// SetSupportDone ticks or unticks a support step.
// @Summary  Tick support step  @Tags admin  @Security BearerAuth  @Router /admin/support/{itemId} [patch]
func (h *LearnerCareHandler) SetSupportDone(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("itemId"))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid step ID")
	}
	var b struct {
		Done bool `json:"done"`
	}
	if err := c.BodyParser(&b); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	ctx, cancel := careCtx()
	defer cancel()
	if e := h.store.SetItemDone(ctx, id, b.Done); e != nil {
		return utils.SendError(c, fiber.StatusNotFound, "Step not found")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Step updated", nil)
}

// DeleteSupport removes a support step.
// @Summary  Delete support step  @Tags admin  @Security BearerAuth  @Router /admin/support/{itemId} [delete]
func (h *LearnerCareHandler) DeleteSupport(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("itemId"))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid step ID")
	}
	ctx, cancel := careCtx()
	defer cancel()
	if e := h.store.DeleteItem(ctx, id); e != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error deleting step")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Step deleted", nil)
}

// Ack hides an automatic support reason for 7 days.
// @Summary  Acknowledge support reason  @Tags admin  @Security BearerAuth  @Router /admin/learners/{id}/support/ack [post]
func (h *LearnerCareHandler) Ack(c *fiber.Ctx) error {
	id, ok, err := h.learner(c)
	if !ok {
		return err
	}
	var b struct {
		ReasonKey string `json:"reason_key"`
	}
	if e := c.BodyParser(&b); e != nil || strings.TrimSpace(b.ReasonKey) == "" || b.ReasonKey == "plan" {
		return utils.SendError(c, fiber.StatusBadRequest, "A reason key is required")
	}
	_, name := h.me(c)
	a := &domain.SupportAck{UserID: id, ReasonKey: b.ReasonKey, Until: time.Now().Add(7 * 24 * time.Hour), By: name}
	ctx, cancel := careCtx()
	defer cancel()
	if e := h.store.Ack(ctx, a); e != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error saving")
	}
	return utils.SendResponse(c, fiber.StatusCreated, "Acknowledged", a)
}
