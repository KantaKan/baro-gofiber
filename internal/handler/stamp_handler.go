package handler

import (
	"fmt"
	"log"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/user"
	"gofiber-baro/internal/storage"
	middleware "gofiber-baro/pkg/middleware"
	"gofiber-baro/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var allowedImageContentTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/webp": true,
	"image/gif":  true,
}

type StampHandler struct {
	repo        domain.StampRepository
	cohortRepo  domain.CohortRepository
	userService *user.Service
	storage     storage.Storage
}

func NewStampHandler(repo domain.StampRepository, cohortRepo domain.CohortRepository, userService *user.Service, s storage.Storage) *StampHandler {
	return &StampHandler{
		repo:        repo,
		cohortRepo:  cohortRepo,
		userService: userService,
		storage:     s,
	}
}

func (h *StampHandler) ListCohorts(c *fiber.Ctx) error {
	cohorts, err := h.cohortRepo.List(c.Context())
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching cohorts")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Cohorts retrieved", cohorts)
}

func (h *StampHandler) GetCohort(c *fiber.Ctx) error {
	cohort, err := c.ParamsInt("cohortNumber", 0)
	if err != nil || cohort <= 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}

	if err := h.enforceCohortAccess(c, cohort); err != nil {
		return err
	}

	doc, err := h.cohortRepo.EnsureExists(c.Context(), cohort)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading cohort")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Cohort retrieved", doc)
}

func (h *StampHandler) GetCohortStamps(c *fiber.Ctx) error {
	cohort, err := c.ParamsInt("cohortNumber", 0)
	if err != nil || cohort <= 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}

	if err := h.enforceCohortAccess(c, cohort); err != nil {
		return err
	}

	if _, err := h.cohortRepo.EnsureExists(c.Context(), cohort); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading cohort")
	}

	stamps, err := h.repo.FindByCohort(c.Context(), cohort)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error fetching stamps")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Stamps retrieved", stamps)
}

func (h *StampHandler) CreateStamp(c *fiber.Ctx) error {
	userID := c.Locals("userID")
	if userID == nil {
		return utils.SendError(c, fiber.StatusUnauthorized, "Unauthorized")
	}

	if h.storage == nil {
		return utils.SendError(c, fiber.StatusServiceUnavailable, "Image storage is not configured")
	}

	userData, err := h.userService.GetUserByID(userID.(string))
	if err != nil {
		return utils.SendError(c, fiber.StatusNotFound, "User not found")
	}

	cohort, err := h.cohortRepo.EnsureExists(c.Context(), userData.CohortNumber)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading cohort")
	}
	if cohort.IsLocked {
		return utils.SendError(c, fiber.StatusForbidden, "This cohort board is locked")
	}

	todayStart := utils.StartOfDay(utils.GetThailandTime())
	alreadyStamped, err := h.repo.HasStampAfter(c.Context(), userData.ID, todayStart)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error checking today's stamp")
	}
	if alreadyStamped {
		return utils.SendError(c, fiber.StatusConflict, "You've already stamped today")
	}

	fileHeader, err := c.FormFile("image")
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Image is required")
	}

	contentType := fileHeader.Header.Get("Content-Type")
	if !allowedImageContentTypes[contentType] {
		return utils.SendError(c, fiber.StatusBadRequest, "File must be a PNG, JPEG, WebP, or GIF image")
	}
	if fileHeader.Size > 4<<20 {
		return utils.SendError(c, fiber.StatusBadRequest, "Image too large")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Could not read image")
	}
	defer file.Close()

	key := fmt.Sprintf("stamps/%d/%s.webp", userData.CohortNumber, primitive.NewObjectID().Hex())
	url, err := h.storage.Upload(c.Context(), key, file, contentType)
	if err != nil {
		log.Printf("ERROR: stamp upload failed: %v", err)
		return utils.SendError(c, fiber.StatusInternalServerError, "Error uploading image")
	}

	stamp := &domain.Stamp{
		OwnerID:      userData.ID,
		CohortNumber: userData.CohortNumber,
		ImageURL:     url,
		CreatedAt:    time.Now(),
	}

	if err := h.repo.InsertStamp(c.Context(), stamp); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error saving stamp")
	}

	return utils.SendResponse(c, fiber.StatusCreated, "Stamp added", stamp)
}

func (h *StampHandler) SetCohortLockAt(c *fiber.Ctx) error {
	cohort, err := c.ParamsInt("cohortNumber", 0)
	if err != nil || cohort <= 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}

	type RequestBody struct {
		Name      *string    `json:"name"`
		LockAt    *time.Time `json:"lockAt"`
		IsLocked  *bool      `json:"isLocked"`
	}

	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid request body")
	}

	if _, err := h.cohortRepo.EnsureExists(c.Context(), cohort); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading cohort")
	}

	set := bson.M{}
	if body.Name != nil {
		set["name"] = *body.Name
	}
	if body.LockAt != nil {
		set["lock_at"] = *body.LockAt
	}
	if body.IsLocked != nil {
		set["is_locked"] = *body.IsLocked
	}

	if err := h.cohortRepo.Update(c.Context(), cohort, bson.M{"$set": set}); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error updating cohort")
	}

	doc, err := h.cohortRepo.FindByCohortNumber(c.Context(), cohort)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading cohort")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Cohort updated", doc)
}

func (h *StampHandler) UploadPoster(c *fiber.Ctx) error {
	cohort, err := c.ParamsInt("cohortNumber", 0)
	if err != nil || cohort <= 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}

	if h.storage == nil {
		return utils.SendError(c, fiber.StatusServiceUnavailable, "Image storage is not configured")
	}

	fileHeader, err := c.FormFile("image")
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Image is required")
	}

	contentType := fileHeader.Header.Get("Content-Type")
	if !allowedImageContentTypes[contentType] {
		return utils.SendError(c, fiber.StatusBadRequest, "File must be a PNG, JPEG, WebP, or GIF image")
	}
	if fileHeader.Size > 10<<20 {
		return utils.SendError(c, fiber.StatusBadRequest, "Image too large")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Could not read image")
	}
	defer file.Close()

	if _, err := h.cohortRepo.EnsureExists(c.Context(), cohort); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading cohort")
	}

	key := fmt.Sprintf("posters/%d.webp", cohort)
	url, err := h.storage.Upload(c.Context(), key, file, contentType)
	if err != nil {
		log.Printf("ERROR: poster upload failed: %v", err)
		return utils.SendError(c, fiber.StatusInternalServerError, "Error uploading image")
	}

	if err := h.cohortRepo.Update(c.Context(), cohort, bson.M{"$set": bson.M{"poster_url": url}}); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error saving poster")
	}

	doc, err := h.cohortRepo.FindByCohortNumber(c.Context(), cohort)
	if err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error loading cohort")
	}
	return utils.SendResponse(c, fiber.StatusOK, "Poster saved", doc)
}

func (h *StampHandler) ClearCohortStamps(c *fiber.Ctx) error {
	cohort, err := c.ParamsInt("cohortNumber", 0)
	if err != nil || cohort <= 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}

	if h.storage != nil {
		if err := h.storage.DeleteObjectsByPrefix(c.Context(), fmt.Sprintf("stamps/%d/", cohort)); err != nil {
			log.Printf("WARNING: failed to delete stamp objects: %v", err)
		}
	}

	if err := h.repo.DeleteByCohort(c.Context(), cohort); err != nil {
		return utils.SendError(c, fiber.StatusInternalServerError, "Error clearing stamps")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Stamps cleared", nil)
}

func (h *StampHandler) DeleteStamp(c *fiber.Ctx) error {
	cohort, err := c.ParamsInt("cohortNumber", 0)
	if err != nil || cohort <= 0 {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid cohort")
	}

	stampID, err := primitive.ObjectIDFromHex(c.Params("stampId"))
	if err != nil {
		return utils.SendError(c, fiber.StatusBadRequest, "Invalid stamp id")
	}

	if h.storage != nil {
		key := fmt.Sprintf("stamps/%d/%s.webp", cohort, stampID.Hex())
		if err := h.storage.DeleteObjectsByPrefix(c.Context(), key); err != nil {
			log.Printf("WARNING: failed to delete stamp object: %v", err)
		}
	}

	if err := h.repo.DeleteByID(c.Context(), cohort, stampID); err != nil {
		if err == mongo.ErrNoDocuments {
			return utils.SendError(c, fiber.StatusNotFound, "Stamp not found")
		}
		return utils.SendError(c, fiber.StatusInternalServerError, "Error deleting stamp")
	}

	return utils.SendResponse(c, fiber.StatusOK, "Stamp deleted", nil)
}

func (h *StampHandler) enforceCohortAccess(c *fiber.Ctx, cohort int) error {
	userID := c.Locals("userID")
	userRole := ""
	if user, ok := c.Locals("user").(*middleware.Claims); ok {
		userRole = user.Role
	}

	if userRole == "admin" {
		return nil
	}

	if userID == nil {
		return utils.SendError(c, fiber.StatusUnauthorized, "Unauthorized")
	}

	userData, err := h.userService.GetUserByID(userID.(string))
	if err != nil {
		return utils.SendError(c, fiber.StatusUnauthorized, "User data not found")
	}
	if cohort != userData.CohortNumber {
		return utils.SendError(c, fiber.StatusForbidden, "You cannot access other cohorts")
	}
	return nil
}
