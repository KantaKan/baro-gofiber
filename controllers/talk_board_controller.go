package controllers

import (
	middleware "gofiber-baro/middlewares"
	"gofiber-baro/models"
	"gofiber-baro/services"
	"gofiber-baro/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func CreatePost(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}

	var input struct {
		Content string `json:"content"`
	}

	if err := c.BodyParser(&input); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	user, err := services.GetUserByID(claims.UserID.Hex())
	if err != nil {
		return utils.SendError(c, fiber.StatusNotFound, "User not found")
	}

	post := &models.Post{
		UserID:   claims.UserID,
		ZoomName: user.ZoomName,
		Cohort:   user.CohortNumber,
		Content:  input.Content,
	}

	createdPost, err := services.CreatePost(post)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error creating post")
	}

	return utils.SendResponse(c, fiber.StatusCreated, "Post created successfully", createdPost)
}

func GetPosts(c *fiber.Ctx) error {
	posts, err := services.GetPosts()
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error getting posts")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Posts retrieved successfully", posts)
}

func AddComment(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}

	postId, err := primitive.ObjectIDFromHex(c.Params("postId"))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid post ID")
	}

	var input struct {
		Content string `json:"content"`
	}

	if err := c.BodyParser(&input); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	user, err := services.GetUserByID(claims.UserID.Hex())
	if err != nil {
		return utils.SendError(c, fiber.StatusNotFound, "User not found")
	}

	comment := &models.Comment{
		UserID:   claims.UserID,
		ZoomName: user.ZoomName,
		Cohort:   user.CohortNumber,
		Content:  input.Content,
	}

	updatedPost, err := services.AddCommentToPost(postId, comment)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error adding comment")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Comment added successfully", updatedPost)
}

func AddReactionToPost(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}

	postId, err := primitive.ObjectIDFromHex(c.Params("postId"))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid post ID")
	}

	var input struct {
		Reaction string `json:"reaction"`
	}

	if err := c.BodyParser(&input); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	reaction := models.Reaction{
		UserID: claims.UserID,
		Type:   "image", // Assuming all reactions from this endpoint are images
		Value:  input.Reaction,
	}

	updatedPost, err := services.AddReactionToPost(postId, &reaction)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error adding reaction")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Reaction added successfully", updatedPost)
}

func AddReactionToComment(c *fiber.Ctx) error {
	claims, ok := c.Locals("user").(*middleware.Claims)
	if !ok {
		return utils.SendError(c, fiber.StatusUnauthorized, "Invalid token claims")
	}

	postId, err := primitive.ObjectIDFromHex(c.Params("postId"))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid post ID")
	}

	commentId, err := primitive.ObjectIDFromHex(c.Params("commentId"))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid comment ID")
	}

	var reaction models.Reaction
	if err := c.BodyParser(&reaction); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}
	reaction.UserID = claims.UserID

	updatedPost, err := services.AddReactionToComment(postId, commentId, &reaction)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error adding reaction to comment")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Reaction added successfully", updatedPost)
}

func GetPost(c *fiber.Ctx) error {
	postID, err := primitive.ObjectIDFromHex(c.Params("postId"))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid post ID")
	}

	post, err := services.GetPostByID(postID)
	if err != nil {
		return utils.SendError(c, fiber.StatusNotFound, "Post not found")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Post retrieved successfully", post)
}