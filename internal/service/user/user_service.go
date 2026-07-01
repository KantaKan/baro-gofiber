package user

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Service struct {
	repo domain.UserRepository
}

func NewService(repo domain.UserRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetUserByID(id string) (*domain.User, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, domain.ErrUserNotFound
	}

	ctx := context.Background()
	return s.repo.FindByID(ctx, oid)
}

func (s *Service) GetUserByEmail(email string) (*domain.User, error) {
	ctx := context.Background()
	return s.repo.FindByEmail(ctx, email)
}

func (s *Service) GetAllUsers(cohort int, role, email, search, sort string, sortDir, page, limit int, excludeAttendanceStatus ...string) ([]domain.User, int, error) {
	ctx := context.Background()

	filter := domain.UserFilter{
		Cohort: cohort,
		Role:   role,
		Email:  email,
		Search: search,
	}

	if len(excludeAttendanceStatus) > 0 && excludeAttendanceStatus[0] != "" {
		filter.ExcludeAttendanceStatus = excludeAttendanceStatus[0]
	}

	findOpts := options.Find()
	if limit > 0 {
		findOpts.SetLimit(int64(limit))
	}
	if page > 1 {
		skip := int64((page - 1) * limit)
		findOpts.SetSkip(skip)
	}
	if sort != "" {
		direction := 1
		if sortDir == -1 {
			direction = -1
		}
		findOpts.SetSort(bson.D{{Key: sort, Value: direction}})
	}

	return s.repo.FindAll(ctx, filter, findOpts)
}

func (s *Service) UpdateUser(id string, update interface{}) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return domain.ErrUserNotFound
	}

	ctx := context.Background()
	return s.repo.Update(ctx, oid, update)
}

func (s *Service) AwardBadge(userID primitive.ObjectID, badgeType, badgeName, emoji, imageUrl, color, style string) error {
	ctx := context.Background()

	badge := domain.Badge{
		ID:        primitive.NewObjectID(),
		Type:      badgeType,
		Name:      badgeName,
		Emoji:     emoji,
		ImageUrl:  imageUrl,
		Color:     color,
		Style:     style,
		AwardedAt: time.Now(),
	}

	return s.repo.AddBadge(ctx, userID, badge)
}

func (s *Service) UpdateReflectionFeedback(userID, reflectionID primitive.ObjectID, feedback string) error {
	ctx := context.Background()
	return s.repo.UpdateReflectionFeedback(ctx, userID, reflectionID, feedback)
}

func (s *Service) CreateReflection(userID primitive.ObjectID, reflection domain.Reflection) (*domain.Reflection, error) {
	ctx := context.Background()

	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, domain.ErrUserNotFound
	}

	now := utils.GetThailandTime()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for _, r := range user.Reflections {
		reflectionDate := time.Date(r.CreatedAt.Year(), r.CreatedAt.Month(), r.CreatedAt.Day(), 0, 0, 0, 0, now.Location())
		if reflectionDate.Equal(today) {
			return nil, errors.New("user has already created a reflection today")
		}
	}

	reflection.CreatedAt = now
	reflection.ID = primitive.NewObjectID()

	if err := s.repo.CreateReflection(ctx, userID, reflection); err != nil {
		return nil, err
	}

	return &reflection, nil
}

func (s *Service) GetReflections(userID primitive.ObjectID) ([]domain.Reflection, error) {
	ctx := context.Background()

	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, domain.ErrUserNotFound
	}

	return user.Reflections, nil
}

func (s *Service) AddProfileComment(userID primitive.ObjectID, commenterID primitive.ObjectID, zoomName string, cohort int, content string, parentID string) error {
	ctx := context.Background()

	comment := domain.ProfileComment{
		ID:        primitive.NewObjectID(),
		UserID:    commenterID,
		ZoomName:  zoomName,
		Cohort:    cohort,
		Content:   content,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if parentID != "" {
		oid, err := primitive.ObjectIDFromHex(parentID)
		if err == nil {
			comment.ParentID = &oid
		}
	}

	return s.repo.AddProfileComment(ctx, userID, comment)
}

func (s *Service) DeleteProfileComment(userID primitive.ObjectID, commentID primitive.ObjectID) error {
	ctx := context.Background()
	return s.repo.DeleteProfileComment(ctx, userID, commentID)
}

func (s *Service) AddProfileReaction(userID primitive.ObjectID, reactorID primitive.ObjectID, reactionType, value string) error {
	ctx := context.Background()

	reaction := domain.Reaction{
		ID:        primitive.NewObjectID(),
		UserID:    reactorID,
		Type:      reactionType,
		Value:     value,
		CreatedAt: time.Now(),
	}

	return s.repo.AddProfileReaction(ctx, userID, reaction)
}

// ponytail: input for bulk-register per-user data
type BulkUserInput struct {
	FirstName    string
	LastName     string
	Email        string
	JSDNumber    string
	Password     string // per-user override
	ProjectGroup string
	GenmateGroup string
	ZoomName     string
}

// ponytail: result for a single bulk-register attempt
type BulkUserResult struct {
	Email    string `json:"email"`
	Status   string `json:"status"`
	Password string `json:"password,omitempty"`
	Error    string `json:"error,omitempty"`
}

var passwordChars = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func generateRandomPassword(n int) string {
	b := make([]rune, n)
	for i := range b {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(passwordChars))))
		b[i] = passwordChars[idx.Int64()]
	}
	return string(b)
}

// ponytail: no per-user tx rollback — skip duplicates, keep going
func (s *Service) BulkCreateUsers(inputs []BulkUserInput, cohortNumber int, sharedPassword string) ([]BulkUserResult, error) {
	ctx := context.Background()
	var results []BulkUserResult
	seenEmails := map[string]bool{}

	for _, in := range inputs {
		if in.Email == "" || in.FirstName == "" || in.LastName == "" {
			results = append(results, BulkUserResult{Email: in.Email, Status: "skipped", Error: "missing required fields"})
			continue
		}

		if seenEmails[in.Email] {
			results = append(results, BulkUserResult{Email: in.Email, Status: "skipped", Error: "duplicate in batch"})
			continue
		}
		seenEmails[in.Email] = true

		existing, err := s.repo.FindByEmail(ctx, in.Email)
		if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
			results = append(results, BulkUserResult{Email: in.Email, Status: "error", Error: "db error: " + err.Error()})
			continue
		}
		if existing != nil {
			results = append(results, BulkUserResult{Email: in.Email, Status: "skipped", Error: "email already registered"})
			continue
		}

		password := in.Password
		if password == "" {
			password = sharedPassword
		}
		if password == "" {
			password = generateRandomPassword(10)
		}

		hashed, err := utils.HashPassword(password)
		if err != nil {
			results = append(results, BulkUserResult{Email: in.Email, Status: "error", Error: "failed to hash password"})
			continue
		}

		user := &domain.User{
			FirstName:    in.FirstName,
			LastName:     in.LastName,
			Email:        in.Email,
			JSDNumber:    in.JSDNumber,
			Password:     hashed,
			Role:         "learner",
			CohortNumber: cohortNumber,
			ProjectGroup: in.ProjectGroup,
			GenmateGroup: in.GenmateGroup,
			ZoomName:     in.ZoomName,
			Reflections:  []domain.Reflection{},
		}

		if err := s.repo.Create(ctx, user); err != nil {
			results = append(results, BulkUserResult{Email: in.Email, Status: "error", Error: "failed to create user"})
			continue
		}

		results = append(results, BulkUserResult{Email: in.Email, Status: "created", Password: password})
	}

	return results, nil
}

func (s *Service) SoftDeleteUser(id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return domain.ErrUserNotFound
	}

	ctx := context.Background()
	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"deleted":    true,
			"deleted_at": now,
		},
	}
	return s.repo.Update(ctx, oid, update)
}
