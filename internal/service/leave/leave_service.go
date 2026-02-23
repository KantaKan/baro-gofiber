package leave

import (
	"context"
	"time"

	"gofiber-baro/internal/domain"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Service struct {
	leaveRepo   domain.LeaveRequestRepository
	userService UserServiceInterface
}

type UserServiceInterface interface {
	GetUserByID(id string) (*domain.User, error)
}

func NewService(leaveRepo domain.LeaveRequestRepository, userService UserServiceInterface) *Service {
	return &Service{
		leaveRepo:   leaveRepo,
		userService: userService,
	}
}

func (s *Service) CreateLeaveRequest(userID primitive.ObjectID, leaveType domain.LeaveType, session *domain.AttendanceSession, date, reason string, isManualEntry bool, createdBy string) (*domain.LeaveRequest, error) {
	ctx := context.Background()

	user, err := s.userService.GetUserByID(userID.Hex())
	if err != nil {
		return nil, err
	}

	request := &domain.LeaveRequest{
		UserID:        userID,
		JSDNumber:     user.JSDNumber,
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		CohortNumber:  user.CohortNumber,
		Type:          leaveType,
		Session:       session,
		Date:          date,
		Reason:        reason,
		Status:        domain.LeaveStatusPending,
		CreatedAt:     time.Now(),
		CreatedBy:     createdBy,
		IsManualEntry: isManualEntry,
	}

	if isManualEntry {
		request.Status = domain.LeaveStatusApproved
	}

	if err := s.leaveRepo.Insert(ctx, request); err != nil {
		return nil, err
	}

	return request, nil
}

func (s *Service) GetLeaveRequests(filter domain.LeaveRequestFilter) ([]domain.LeaveRequest, error) {
	ctx := context.Background()
	return s.leaveRepo.FindAll(ctx, filter)
}

func (s *Service) GetMyLeaveRequests(userID primitive.ObjectID) ([]domain.LeaveRequest, error) {
	ctx := context.Background()
	return s.leaveRepo.FindByUserID(ctx, userID)
}

func (s *Service) UpdateLeaveRequestStatus(id primitive.ObjectID, status domain.LeaveRequestStatus, reviewedBy primitive.ObjectID, reviewedByName, reviewNotes string) error {
	ctx := context.Background()
	return s.leaveRepo.UpdateStatus(ctx, id, status, reviewedBy, reviewedByName, reviewNotes)
}

func (s *Service) GetLeaveRequestByID(id primitive.ObjectID) (*domain.LeaveRequest, error) {
	ctx := context.Background()
	return s.leaveRepo.FindByID(ctx, id)
}
