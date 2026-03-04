package handler

import (
	"log"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TalkBoardHandler struct {
	repo domain.TalkBoardRepository
}

func NewTalkBoardHandler(repo domain.TalkBoardRepository) *TalkBoardHandler {
	return &TalkBoardHandler{repo: repo}
}

func (h *TalkBoardHandler) GetPosts(c *fiber.Ctx) error {
	ctx := c.Context()
	cohort := c.QueryInt("cohort", 0)

	filter := domain.PostFilter{Cohort: cohort}
	posts, err := h.repo.FindPosts(ctx, filter, nil)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching posts")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Posts retrieved", posts)
}

func (h *TalkBoardHandler) GetPost(c *fiber.Ctx) error {
	ctx := c.Context()
	id := c.Params("postId")
	if id == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Post ID is required")
	}

	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid post ID")
	}

	post, err := h.repo.FindByID(ctx, oid)
	if err != nil {
		return utils.SendError(c, fiber.StatusNotFound, "Post not found")
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
		ZoomName string `json:"zoomName"`
		Cohort   int    `json:"cohort"`
		Content  string `json:"content"`
	}

	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if body.Content == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Content is required")
	}

	userOID, _ := primitive.ObjectIDFromHex(userID.(string))

	post := &domain.Post{
		UserID:    userOID,
		ZoomName:  body.ZoomName,
		Cohort:    body.Cohort,
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

	type RequestBody struct {
		ZoomName string `json:"zoomName"`
		Cohort   int    `json:"cohort"`
		Content  string `json:"content"`
	}

	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if body.Content == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Content is required")
	}

	userOID, _ := primitive.ObjectIDFromHex(userID.(string))

	comment := domain.Comment{
		ID:        primitive.NewObjectID(),
		UserID:    userOID,
		ZoomName:  body.ZoomName,
		Cohort:    body.Cohort,
		Content:   body.Content,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.repo.AddComment(ctx, postOID, comment); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error adding comment")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Comment added", comment)
}

func (h *TalkBoardHandler) AddReactionToPost(c *fiber.Ctx) error {
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
		log.Printf("ERROR: Invalid post ID format: %v", err)
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid post ID")
	}

	// Verify post exists
	exists, err := h.repo.Exists(ctx, postOID)
	if err != nil {
		log.Printf("ERROR: Error checking post existence: %v", err)
		return utils.SendError(c, fiber.StatusInternalServerError, "Error checking post")
	}
	if !exists {
		log.Printf("ERROR: Post not found: %s", postID)
		return utils.SendError(c, fiber.StatusNotFound, "Post not found")
	}

	type RequestBody struct {
		Reaction string `json:"reaction"`
	}

	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		log.Printf("ERROR: Body parser error: %v", err)
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if body.Reaction == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Reaction is required")
	}

	log.Printf("DEBUG: Adding reaction %s to post %s by user %s", body.Reaction, postID, userID.(string))

	userOID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		log.Printf("ERROR: Invalid user ID format: %v, userID: %v", err, userID)
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID format")
	}

	// First ensure reactions field exists and is an array
	initUpdate := bson.M{
		"$setOnInsert": bson.M{
			"reactions": []domain.Reaction{},
		},
	}
	err = h.repo.UpdatePost(ctx, postOID, initUpdate)
	if err != nil {
		log.Printf("DEBUG: initUpdate result: %v", err)
	}

	// Now remove any existing reaction by this user
	pullUpdate := bson.M{"$pull": bson.M{"reactions": bson.M{"userId": userOID}}}
	if err := h.repo.UpdatePost(ctx, postOID, pullUpdate); err != nil {
		log.Printf("DEBUG: pullUpdate result: %v", err)
		// Continue anyway - might be first reaction
	}

	reaction := domain.Reaction{
		ID:        primitive.NewObjectID(),
		UserID:    userOID,
		Type:      "emoji",
		Value:     body.Reaction,
		CreatedAt: time.Now(),
	}

	log.Printf("DEBUG: Adding reaction to MongoDB: postID=%s, reaction=%+v", postOID.Hex(), reaction)
	err = h.repo.AddReaction(ctx, postOID, reaction)
	if err != nil {
		log.Printf("ERROR: Error adding reaction: %v", err)
		return utils.SendError(c, fiber.StatusInternalServerError, "Error adding reaction: "+err.Error())
	}

	return utils.SendResponse(c, fiber.StatusOK, "Reaction added", reaction)
}

func (h *TalkBoardHandler) RemoveReactionFromPost(c *fiber.Ctx) error {
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

	userOID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid user ID format")
	}

	// Remove reaction where userId matches
	update := bson.M{"$pull": bson.M{"reactions": bson.M{"userId": userOID}}}
	if err := h.repo.UpdatePost(ctx, postOID, update); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error removing reaction")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Reaction removed", nil)
}

func (h *TalkBoardHandler) AddReactionToComment(c *fiber.Ctx) error {
	userID := c.Locals("userID")
	if userID == nil {
		return utils.SendError(c, fiber.StatusUnauthorized, "Unauthorized")
	}

	commentID := c.Params("commentId")
	if commentID == "" {
		return utils.SendError(c, fiber.StatusBadRequest, "Comment ID is required")
	}

	commentOID, err := primitive.ObjectIDFromHex(commentID)
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid comment ID")
	}

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

	userOID, _ := primitive.ObjectIDFromHex(userID.(string))

	reaction := domain.Reaction{
		ID:        primitive.NewObjectID(),
		UserID:    userOID,
		Type:      "emoji",
		Value:     body.Reaction,
		CreatedAt: time.Now(),
	}

	_ = commentOID
	return utils.SendResponse(c, fiber.StatusOK, "Reaction added", reaction)
}
