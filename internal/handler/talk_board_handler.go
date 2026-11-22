package handler

import (
	"gofiber-baro/internal/service/staff"
	"html"
	"log"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/user"
	middleware "gofiber-baro/pkg/middleware"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TalkBoardHandler struct {
	repo        domain.TalkBoardRepository
	userService boardUsers
}

func NewTalkBoardHandler(repo domain.TalkBoardRepository, userService *user.Service) *TalkBoardHandler {
	return &TalkBoardHandler{
		repo:        repo,
		userService: userService,
	}
}

func (h *TalkBoardHandler) GetPosts(c *fiber.Ctx) error {
	ctx := c.Context()
	// Learners always get their own cohort; staff only cohorts in their scope.
	a, ok := h.actor(c)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "User data not found")
	}
	cohort, allowed := boardCohortForRead(a, c.QueryInt("cohort", 0))
	if !allowed {
		return utils.SendError(c, fiber.StatusForbidden, "You don't have access to this cohort's board")
	}

	filter := domain.PostFilter{Cohort: cohort}
	posts, err := h.repo.FindPosts(ctx, filter, nil)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching posts")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Posts retrieved", posts)
}

func (h *TalkBoardHandler) GetPost(c *fiber.Ctx) error {
	post, _, ok := h.postFor(c, false)
	if !ok {
		return nil
	}
	return utils.SendResponse(c, fiber.StatusOK, "Post retrieved", post)
}

func (h *TalkBoardHandler) CreatePost(c *fiber.Ctx) error {
	ctx := c.Context()
	userID := c.Locals("userID")
	if userID == nil {
		return utils.SendError(c, fiber.StatusUnauthorized, "Unauthorized")
	}

	type RequestBody struct {
		Content string `json:"content"`
		Cohort  int    `json:"cohort"` // staff only: which cohort's board (learners always post to their own)
	}

	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if body.Content == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Content is required")
	}

	userData, err := h.userService.GetUserByID(userID.(string))
	if err != nil {
		return utils.SendError(c, fiber.StatusNotFound, "User not found")
	}
	target, allowed := boardCohortForPost(boardActor{role: staff.Effective(userData.Role, userData.StaffRole), cohorts: userData.StaffCohorts, ownCohort: userData.CohortNumber}, body.Cohort)
	if !allowed {
		return utils.SendError(c, fiber.StatusForbidden, "You can't post to that cohort's board")
	}

	userOID, _ := primitive.ObjectIDFromHex(userID.(string))

	post := &domain.Post{
		UserID:    userOID,
		ZoomName:  boardName(userData),
		Cohort:    target,
		Content:   body.Content,
		Reactions: []domain.Reaction{},
		Comments:  []domain.Comment{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.repo.InsertPost(ctx, post); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error creating post")
	}

	return utils.SendResponse(c, fiber.StatusCreated, "Post created", post)
}

func (h *TalkBoardHandler) AddComment(c *fiber.Ctx) error {
	type RequestBody struct {
		Content string `json:"content"`
	}
	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	if body.Content == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Content is required")
	}

	post, _, ok := h.postFor(c, true)
	if !ok {
		return nil
	}
	userID, _ := c.Locals("userID").(string)
	userData, err := h.userService.GetUserByID(userID)
	if err != nil {
		return utils.SendError(c, fiber.StatusNotFound, "User not found")
	}
	userOID, _ := primitive.ObjectIDFromHex(userID)

	comment := domain.Comment{
		ID:       primitive.NewObjectID(),
		UserID:   userOID,
		ZoomName: boardName(userData),
		// a reply starts with an empty list: MongoDB can't push a reaction onto null
		Reactions: []domain.Reaction{},
		// the reply belongs to the post's board (staff have no cohort of their own)
		Cohort:    post.Cohort,
		Content:   body.Content,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := h.repo.AddComment(c.Context(), post.ID, comment); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error adding comment")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Comment added", comment)
}

func (h *TalkBoardHandler) AddReactionToPost(c *fiber.Ctx) error {
	type RequestBody struct {
		Reaction string `json:"reaction"`
	}
	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	if body.Reaction == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Reaction is required")
	}

	post, _, ok := h.postFor(c, true)
	if !ok {
		return nil
	}
	userOID, err := primitive.ObjectIDFromHex(c.Locals("userID").(string))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID format")
	}

	// one reaction per person: drop their old one first
	if err := h.repo.UpdatePost(c.Context(), post.ID, bson.M{"$pull": bson.M{"reactions": bson.M{"userId": userOID}}}); err != nil {
		log.Printf("board: clearing old reaction on %s: %v", post.ID.Hex(), err)
	}
	reaction := domain.Reaction{
		ID:        primitive.NewObjectID(),
		UserID:    userOID,
		Type:      "emoji",
		Value:     html.EscapeString(body.Reaction),
		CreatedAt: time.Now(),
	}
	if err := h.repo.AddReaction(c.Context(), post.ID, reaction); err != nil {
		log.Printf("board: adding reaction to %s: %v", post.ID.Hex(), err)
		return utils.SendError(c, fiber.StatusInternalServerError, "Error adding reaction")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Reaction added", reaction)
}

func (h *TalkBoardHandler) RemoveReactionFromPost(c *fiber.Ctx) error {
	post, _, ok := h.postFor(c, true)
	if !ok {
		return nil
	}
	userOID, err := primitive.ObjectIDFromHex(c.Locals("userID").(string))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID format")
	}
	if err := h.repo.UpdatePost(c.Context(), post.ID, bson.M{"$pull": bson.M{"reactions": bson.M{"userId": userOID}}}); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error removing reaction")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Reaction removed", nil)
}

// replyFor resolves :commentId on a post the caller may write to.
func (h *TalkBoardHandler) replyFor(c *fiber.Ctx) (*domain.Post, primitive.ObjectID, primitive.ObjectID, bool) {
	post, _, ok := h.postFor(c, true)
	if !ok {
		return nil, primitive.NilObjectID, primitive.NilObjectID, false
	}
	commentOID, err := primitive.ObjectIDFromHex(c.Params("commentId"))
	if err != nil {
		_ = utils.SendError(c, fiber.StatusBadRequest, "Invalid comment ID")
		return nil, primitive.NilObjectID, primitive.NilObjectID, false
	}
	found := false
	for _, cm := range post.Comments {
		if cm.ID == commentOID {
			found = true
			break
		}
	}
	if !found {
		_ = utils.SendError(c, fiber.StatusNotFound, "Comment not found")
		return nil, primitive.NilObjectID, primitive.NilObjectID, false
	}
	userOID, err := primitive.ObjectIDFromHex(c.Locals("userID").(string))
	if err != nil {
		_ = utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID format")
		return nil, primitive.NilObjectID, primitive.NilObjectID, false
	}
	return post, commentOID, userOID, true
}

func (h *TalkBoardHandler) AddReactionToComment(c *fiber.Ctx) error {
	type RequestBody struct {
		Reaction string `json:"reaction"`
	}
	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	if body.Reaction == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Reaction is required")
	}

	post, commentOID, userOID, ok := h.replyFor(c)
	if !ok {
		return nil
	}
	// one reaction per person on a reply: drop their old one first
	if err := h.repo.RemoveCommentReaction(c.Context(), post.ID, commentOID, userOID); err != nil {
		log.Printf("board: clearing old reply reaction on %s: %v", commentOID.Hex(), err)
	}
	reaction := domain.Reaction{
		ID:        primitive.NewObjectID(),
		UserID:    userOID,
		Type:      "emoji",
		Value:     html.EscapeString(body.Reaction),
		CreatedAt: time.Now(),
	}
	if err := h.repo.AddCommentReaction(c.Context(), post.ID, commentOID, reaction); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error adding reaction")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Reaction added", reaction)
}

func (h *TalkBoardHandler) RemoveReactionFromComment(c *fiber.Ctx) error {
	post, commentOID, userOID, ok := h.replyFor(c)
	if !ok {
		return nil
	}
	if err := h.repo.RemoveCommentReaction(c.Context(), post.ID, commentOID, userOID); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error removing reaction")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Reaction removed", nil)
}

func (h *TalkBoardHandler) DeletePost(c *fiber.Ctx) error {
	ctx := c.Context()
	userID := c.Locals("userID")
	if userID == nil {
		return utils.SendError(c, fiber.StatusUnauthorized, "Unauthorized")
	}

	postID := c.Params("postId")
	if postID == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Post ID is required")
	}

	postOID, err := primitive.ObjectIDFromHex(postID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid post ID")
	}

	post, err := h.repo.FindByID(ctx, postOID)
	if err != nil {
		return utils.SendError(c, fiber.StatusNotFound, "Post not found")
	}

	currentUserID := ""
	if user, ok := c.Locals("user").(*middleware.Claims); ok {
		currentUserID = user.UserID
	}

	a, _ := h.actor(c)
	isOwner := post.UserID.Hex() == currentUserID

	if !isOwner && !boardMayModerate(a, post.Cohort) {
		return utils.SendError(c, fiber.StatusForbidden, "You can only delete your own posts")
	}

	if err := h.repo.DeletePost(ctx, postOID); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error deleting post")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Post deleted", nil)
}

func (h *TalkBoardHandler) DeleteComment(c *fiber.Ctx) error {
	ctx := c.Context()
	userID := c.Locals("userID")
	if userID == nil {
		return utils.SendError(c, fiber.StatusUnauthorized, "Unauthorized")
	}

	postID := c.Params("postId")
	commentID := c.Params("commentId")

	if postID == "" || commentID == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Post ID and Comment ID are required")
	}

	postOID, err := primitive.ObjectIDFromHex(postID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid post ID")
	}

	commentOID, err := primitive.ObjectIDFromHex(commentID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid comment ID")
	}

	post, err := h.repo.FindByID(ctx, postOID)
	if err != nil {
		return utils.SendError(c, fiber.StatusNotFound, "Post not found")
	}

	var commentFound *domain.Comment
	for i := range post.Comments {
		if post.Comments[i].ID == commentOID {
			commentFound = &post.Comments[i]
			break
		}
	}

	if commentFound == nil {
		return utils.SendError(c, fiber.StatusNotFound, "Comment not found")
	}

	currentUserID := ""
	if user, ok := c.Locals("user").(*middleware.Claims); ok {
		currentUserID = user.UserID
	}

	a, _ := h.actor(c)
	isOwner := commentFound.UserID.Hex() == currentUserID

	if !isOwner && !boardMayModerate(a, post.Cohort) {
		return utils.SendError(c, fiber.StatusForbidden, "You can only delete your own comments")
	}

	if err := h.repo.DeleteComment(ctx, postOID, commentOID); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error deleting comment")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Comment deleted", nil)
}
