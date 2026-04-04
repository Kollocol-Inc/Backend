package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"path/filepath"
	"strings"

	"user-service/constants"
	"user-service/internal/repository"
	"user-service/pkg/errors"
	"user-service/pkg/messaging"
	"user-service/pkg/storage"
	pb "user-service/proto"
	"google.golang.org/grpc/codes"
)

type UserRepo interface {
	GetUserByID(ctx context.Context, userID string) (*repository.User, error)
	GetUserByEmail(ctx context.Context, email string) (*repository.User, error)
	UpdateUser(ctx context.Context, user *repository.User) error
	GetUsersByEmailsMap(ctx context.Context, emails []string) (map[string]*repository.User, error)
}

type SettingsRepo interface {
	GetOrCreateSettings(ctx context.Context, userID string) (*repository.NotificationSettings, error)
	CreateDefaultSettings(ctx context.Context, userID string) (*repository.NotificationSettings, error)
	UpdateSettings(ctx context.Context, settings *repository.NotificationSettings) error
}

type GroupRepo interface {
	CreateGroup(ctx context.Context, name, ownerID string) (*repository.Group, error)
	GetGroupByID(ctx context.Context, groupID string) (*repository.Group, error)
	UpdateGroup(ctx context.Context, group *repository.Group) error
	DeleteGroup(ctx context.Context, groupID string) error
	AddMembers(ctx context.Context, groupID string, userIDs []string) error
	GetMemberIDs(ctx context.Context, groupID string) ([]string, error)
	GetMemberCount(ctx context.Context, groupID string) (int32, error)
	GetUserGroups(ctx context.Context, userID string) ([]*repository.Group, error)
	GetCreatedGroups(ctx context.Context, ownerID string) ([]*repository.Group, error)
	IsMember(ctx context.Context, groupID, userID string) (bool, error)
	GetGroupUsers(ctx context.Context, groupID string) ([]*repository.User, error)
}

type FileStorage interface {
	UploadFile(ctx context.Context, bucketName, objectName string, reader io.Reader, size int64, contentType string) error
	DeleteFile(ctx context.Context, bucketName, objectName string) error
	GetPublicURL(bucketName, objectName string) string
}

type MessagePublisher interface {
	Publish(ctx context.Context, queueName string, body []byte) error
}

type UserService struct {
	pb.UnimplementedUserServiceServer
	userRepo     UserRepo
	settingsRepo SettingsRepo
	groupRepo    GroupRepo
	s3Client     FileStorage
	rabbitMQ     MessagePublisher
}

func NewUserService(
	db *sql.DB,
	s3Client *storage.S3Client,
	rabbitMQ *messaging.RabbitMQClient,
) *UserService {
	return &UserService{
		userRepo:     repository.NewUserRepository(db),
		settingsRepo: repository.NewNotificationSettingsRepository(db),
		groupRepo:    repository.NewGroupRepository(db),
		s3Client:     s3Client,
		rabbitMQ:     rabbitMQ,
	}
}

func NewUserServiceWithDeps(
	userRepo UserRepo,
	settingsRepo SettingsRepo,
	groupRepo GroupRepo,
	s3Client FileStorage,
	rabbitMQ MessagePublisher,
) *UserService {
	return &UserService{
		userRepo:     userRepo,
		settingsRepo: settingsRepo,
		groupRepo:    groupRepo,
		s3Client:     s3Client,
		rabbitMQ:     rabbitMQ,
	}
}

func (s *UserService) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	log.Printf("Register user %s", req.UserId)

	user, err := s.userRepo.GetUserByID(ctx, req.UserId)
	if err != nil {
		log.Printf("Failed to get user by ID %s: %v", req.UserId, err)
		return nil, errors.New(codes.NotFound, errors.ReasonUserNotFound, "User not found", map[string]string{"user_id": req.UserId})
	}

	log.Printf("Found user: %s (email: %s, is_registered: %v)", user.ID, user.Email, user.IsRegistered)

	user.FirstName = req.FirstName
	user.LastName = req.LastName
	user.IsRegistered = true

	if len(req.AvatarData) > 0 && req.AvatarFilename != "" {
		avatarURL, err := s.uploadAvatar(ctx, req.UserId, req.AvatarFilename, req.AvatarData, user.AvatarURL)
		if err != nil {
			log.Printf("Failed to upload avatar: %v", err)
			return nil, errors.New(codes.Internal, errors.ReasonAvatarUploadFailed, "Failed to upload avatar", map[string]string{"user_id": req.UserId})
		}
		user.AvatarURL = avatarURL
	}

	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		log.Printf("Failed to update user: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonUserUpdateFailed, "Failed to update profile", map[string]string{"user_id": req.UserId})
	}

	log.Printf("User %s registered successfully", user.ID)

	_, err = s.settingsRepo.CreateDefaultSettings(ctx, user.ID)
	if err != nil {
		log.Printf("Failed to create notification settings: %v", err)
	}

	return &pb.RegisterResponse{
		User: s.userToProto(user),
	}, nil
}

func (s *UserService) GetProfile(ctx context.Context, req *pb.GetProfileRequest) (*pb.GetProfileResponse, error) {
	user, err := s.userRepo.GetUserByID(ctx, req.UserId)
	if err != nil {
		return nil, errors.New(codes.NotFound, errors.ReasonUserNotFound, "User not found", map[string]string{"user_id": req.UserId})
	}

	return &pb.GetProfileResponse{
		User: s.userToProto(user),
	}, nil
}

func (s *UserService) GetProfileByEmail(ctx context.Context, req *pb.GetProfileByEmailRequest) (*pb.GetProfileByEmailResponse, error) {
	user, err := s.userRepo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, errors.New(codes.NotFound, errors.ReasonUserNotFound, "User not found", map[string]string{"email": req.Email})
	}

	return &pb.GetProfileByEmailResponse{
		User: s.userToProto(user),
	}, nil
}

func (s *UserService) UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest) (*pb.UpdateProfileResponse, error) {
	user, err := s.userRepo.GetUserByID(ctx, req.UserId)
	if err != nil {
		return nil, errors.New(codes.NotFound, errors.ReasonUserNotFound, "User not found", map[string]string{"user_id": req.UserId})
	}

	if req.FirstName != "" {
		user.FirstName = req.FirstName
	}
	if req.LastName != "" {
		user.LastName = req.LastName
	}

	if len(req.AvatarData) > 0 && req.AvatarFilename != "" {
		avatarURL, err := s.uploadAvatar(ctx, req.UserId, req.AvatarFilename, req.AvatarData, user.AvatarURL)
		if err != nil {
			log.Printf("Failed to upload avatar: %v", err)
			return nil, errors.New(codes.Internal, errors.ReasonAvatarUploadFailed, "Failed to upload avatar", map[string]string{"user_id": req.UserId})
		}
		user.AvatarURL = avatarURL
	}

	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		log.Printf("Failed to update user: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonUserUpdateFailed, "Failed to update profile", map[string]string{"user_id": req.UserId})
	}

	return &pb.UpdateProfileResponse{
		User: s.userToProto(user),
	}, nil
}

func (s *UserService) GetNotificationSettings(ctx context.Context, req *pb.GetNotificationSettingsRequest) (*pb.GetNotificationSettingsResponse, error) {
	settings, err := s.settingsRepo.GetOrCreateSettings(ctx, req.UserId)
	if err != nil {
		log.Printf("Failed to get notification settings: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonSettingsNotFound, "Failed to retrieve settings", map[string]string{"user_id": req.UserId})
	}

	return &pb.GetNotificationSettingsResponse{
		Settings: s.settingsToProto(settings),
	}, nil
}

func (s *UserService) UpdateNotificationSettings(ctx context.Context, req *pb.UpdateNotificationSettingsRequest) (*pb.UpdateNotificationSettingsResponse, error) {
	log.Printf("UpdateNotificationSettings for user %s: NewQuizzes=%v, QuizResults=%v, GroupInvites=%v, DeadlineReminder=%v",
		req.UserId, req.NewQuizzes, req.QuizResults, req.GroupInvites, req.DeadlineReminder)

	settings, err := s.settingsRepo.GetOrCreateSettings(ctx, req.UserId)
	if err != nil {
		log.Printf("Failed to get notification settings: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonSettingsNotFound, "Failed to retrieve settings", map[string]string{"user_id": req.UserId})
	}

	log.Printf("Current settings: NewQuizzes=%v, QuizResults=%v, GroupInvites=%v, DeadlineReminder=%s",
		settings.NewQuizzes, settings.QuizResults, settings.GroupInvites, settings.DeadlineReminder)

	if req.NewQuizzes != nil {
		log.Printf("Updating NewQuizzes from %v to %v", settings.NewQuizzes, *req.NewQuizzes)
		settings.NewQuizzes = *req.NewQuizzes
	}
	if req.QuizResults != nil {
		log.Printf("Updating QuizResults from %v to %v", settings.QuizResults, *req.QuizResults)
		settings.QuizResults = *req.QuizResults
	}
	if req.GroupInvites != nil {
		log.Printf("Updating GroupInvites from %v to %v", settings.GroupInvites, *req.GroupInvites)
		settings.GroupInvites = *req.GroupInvites
	}
	if req.DeadlineReminder != nil {
		log.Printf("Updating DeadlineReminder from %s to %s", settings.DeadlineReminder, *req.DeadlineReminder)
		settings.DeadlineReminder = *req.DeadlineReminder
	}

	log.Printf("Updated settings before save: NewQuizzes=%v, QuizResults=%v, GroupInvites=%v, DeadlineReminder=%s",
		settings.NewQuizzes, settings.QuizResults, settings.GroupInvites, settings.DeadlineReminder)

	if err := s.settingsRepo.UpdateSettings(ctx, settings); err != nil {
		log.Printf("Failed to update settings: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonSettingsUpdateFailed, "Failed to update settings", map[string]string{"user_id": req.UserId})
	}

	log.Printf("Settings saved successfully")

	return &pb.UpdateNotificationSettingsResponse{
		Settings: s.settingsToProto(settings),
	}, nil
}

func (s *UserService) CreateGroup(ctx context.Context, req *pb.CreateGroupRequest) (*pb.CreateGroupResponse, error) {
	group, err := s.groupRepo.CreateGroup(ctx, req.Name, req.OwnerId)
	if err != nil {
		log.Printf("Failed to create group: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonGroupCreateFailed, "Failed to create group", map[string]string{"owner_id": req.OwnerId})
	}

	usersMap, err := s.userRepo.GetUsersByEmailsMap(ctx, req.MemberEmails)
	if err != nil {
		log.Printf("Failed to get users by emails: %v", err)
	}

	userIDs := []string{req.OwnerId}
	addedEmails := []string{}
	for _, email := range req.MemberEmails {
		user, exists := usersMap[email]
		if !exists {
			log.Printf("User not found for email %s", email)
			continue
		}
		userIDs = append(userIDs, user.ID)
		addedEmails = append(addedEmails, email)
	}

	if err := s.groupRepo.AddMembers(ctx, group.ID, userIDs); err != nil {
		log.Printf("Failed to add members: %v", err)
	}

	for _, email := range addedEmails {
		s.publishGroupInvite(ctx, group.ID, group.Name, req.OwnerId, email)
	}

	memberCount, _ := s.groupRepo.GetMemberCount(ctx, group.ID)
	group.MemberCount = memberCount

	return &pb.CreateGroupResponse{
		Group: s.groupToProto(group),
	}, nil
}

func (s *UserService) GetGroups(ctx context.Context, req *pb.GetGroupsRequest) (*pb.GetGroupsResponse, error) {
	var groups []*repository.Group
	var err error

	switch req.Filter {
	case "created":
		groups, err = s.groupRepo.GetCreatedGroups(ctx, req.UserId)
	case "my":
		groups, err = s.groupRepo.GetUserGroups(ctx, req.UserId)
	default:
		return nil, errors.New(codes.InvalidArgument, errors.ReasonInvalidFilter, "Invalid filter", map[string]string{"filter": req.Filter})
	}

	if err != nil {
		log.Printf("Failed to get groups: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonGroupNotFound, "Failed to retrieve groups", map[string]string{"user_id": req.UserId})
	}

	protoGroups := make([]*pb.Group, len(groups))
	for i, group := range groups {
		protoGroups[i] = s.groupToProto(group)
	}

	return &pb.GetGroupsResponse{
		Groups: protoGroups,
	}, nil
}

func (s *UserService) GetGroup(ctx context.Context, req *pb.GetGroupRequest) (*pb.GetGroupResponse, error) {
	group, err := s.groupRepo.GetGroupByID(ctx, req.GroupId)
	if err != nil {
		return nil, errors.New(codes.NotFound, errors.ReasonGroupNotFound, "Group not found", map[string]string{"group_id": req.GroupId})
	}

	isMember, err := s.groupRepo.IsMember(ctx, req.GroupId, req.UserId)
	if err != nil || (!isMember && group.OwnerID != req.UserId) {
		return nil, errors.New(codes.PermissionDenied, errors.ReasonAccessDenied, "Access denied", map[string]string{"group_id": req.GroupId, "user_id": req.UserId})
	}

	users, err := s.groupRepo.GetGroupUsers(ctx, req.GroupId)
	if err != nil {
		log.Printf("Failed to get group users: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonMembersRetrieveFailed, "Failed to retrieve members", map[string]string{"group_id": req.GroupId})
	}

	members := make([]*pb.User, 0, len(users))
	for _, user := range users {
		members = append(members, s.userToProto(user))
	}

	return &pb.GetGroupResponse{
		Group: &pb.GroupWithMembers{
			Group:   s.groupToProto(group),
			Members: members,
		},
	}, nil
}

func (s *UserService) UpdateGroup(ctx context.Context, req *pb.UpdateGroupRequest) (*pb.UpdateGroupResponse, error) {
	group, err := s.groupRepo.GetGroupByID(ctx, req.GroupId)
	if err != nil {
		return nil, errors.New(codes.NotFound, errors.ReasonGroupNotFound, "Group not found", map[string]string{"group_id": req.GroupId})
	}

	if group.OwnerID != req.UserId {
		return nil, errors.New(codes.PermissionDenied, errors.ReasonUnauthorized, "Only group owner can update group", map[string]string{"group_id": req.GroupId, "user_id": req.UserId})
	}

	if req.Name != "" {
		group.Name = req.Name
		if err := s.groupRepo.UpdateGroup(ctx, group); err != nil {
			log.Printf("Failed to update group: %v", err)
			return nil, errors.New(codes.Internal, errors.ReasonGroupUpdateFailed, "Failed to update group", map[string]string{"group_id": req.GroupId})
		}
	}

	if len(req.MemberEmails) > 0 {
		usersMap, err := s.userRepo.GetUsersByEmailsMap(ctx, req.MemberEmails)
		if err != nil {
			log.Printf("Failed to get users by emails: %v", err)
		}

		existingMemberIDs, err := s.groupRepo.GetMemberIDs(ctx, req.GroupId)
		if err != nil {
			log.Printf("Failed to get existing members: %v", err)
			existingMemberIDs = []string{}
		}
		existingMembersSet := make(map[string]bool, len(existingMemberIDs))
		for _, id := range existingMemberIDs {
			existingMembersSet[id] = true
		}

		newUserIDs := []string{}
		addedEmails := []string{}
		for _, email := range req.MemberEmails {
			user, exists := usersMap[email]
			if !exists {
				log.Printf("User not found for email %s", email)
				continue
			}

			if existingMembersSet[user.ID] {
				continue
			}

			newUserIDs = append(newUserIDs, user.ID)
			addedEmails = append(addedEmails, email)
		}

		if len(newUserIDs) > 0 {
			if err := s.groupRepo.AddMembers(ctx, req.GroupId, newUserIDs); err != nil {
				log.Printf("Failed to add members: %v", err)
			}

			for _, email := range addedEmails {
				s.publishGroupInvite(ctx, group.ID, group.Name, req.UserId, email)
			}
		}
	}

	memberCount, _ := s.groupRepo.GetMemberCount(ctx, req.GroupId)
	group.MemberCount = memberCount

	return &pb.UpdateGroupResponse{
		Group: s.groupToProto(group),
	}, nil
}

func (s *UserService) DeleteGroup(ctx context.Context, req *pb.DeleteGroupRequest) (*pb.DeleteGroupResponse, error) {
	group, err := s.groupRepo.GetGroupByID(ctx, req.GroupId)
	if err != nil {
		return nil, errors.New(codes.NotFound, errors.ReasonGroupNotFound, "Group not found", map[string]string{"group_id": req.GroupId})
	}

	if group.OwnerID != req.UserId {
		return nil, errors.New(codes.PermissionDenied, errors.ReasonUnauthorized, "Only group owner can delete group", map[string]string{"group_id": req.GroupId, "user_id": req.UserId})
	}

	if err := s.groupRepo.DeleteGroup(ctx, req.GroupId); err != nil {
		log.Printf("Failed to delete group: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonGroupDeleteFailed, "Failed to delete group", map[string]string{"group_id": req.GroupId})
	}

	return &pb.DeleteGroupResponse{}, nil
}

func (s *UserService) CheckGroupMembership(ctx context.Context, req *pb.CheckGroupMembershipRequest) (*pb.CheckGroupMembershipResponse, error) {
	isMember, err := s.groupRepo.IsMember(ctx, req.GroupId, req.UserId)
	if err != nil {
		log.Printf("Failed to check group membership: %v", err)
		return &pb.CheckGroupMembershipResponse{
			IsMember: false,
			Role:     "",
		}, nil
	}

	if !isMember {
		return &pb.CheckGroupMembershipResponse{
			IsMember: false,
			Role:     "",
		}, nil
	}

	group, err := s.groupRepo.GetGroupByID(ctx, req.GroupId)
	if err != nil {
		log.Printf("Failed to get group: %v", err)
		return &pb.CheckGroupMembershipResponse{
			IsMember: true,
			Role:     "member",
		}, nil
	}

	role := "member"
	if group.OwnerID == req.UserId {
		role = "owner"
	}

	return &pb.CheckGroupMembershipResponse{
		IsMember: true,
		Role:     role,
	}, nil
}

func (s *UserService) userToProto(user *repository.User) *pb.User {
	return &pb.User{
		Id:           user.ID,
		Email:        user.Email,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		AvatarUrl:    user.AvatarURL,
		IsRegistered: user.IsRegistered,
		CreatedAt:    user.CreatedAt.Unix(),
	}
}

func (s *UserService) settingsToProto(settings *repository.NotificationSettings) *pb.NotificationSettings {
	return &pb.NotificationSettings{
		UserId:           settings.UserID,
		NewQuizzes:       settings.NewQuizzes,
		QuizResults:      settings.QuizResults,
		GroupInvites:     settings.GroupInvites,
		DeadlineReminder: settings.DeadlineReminder,
		UpdatedAt:        settings.UpdatedAt.Unix(),
	}
}

func (s *UserService) groupToProto(group *repository.Group) *pb.Group {
	return &pb.Group{
		Id:          group.ID,
		Name:        group.Name,
		OwnerId:     group.OwnerID,
		CreatedAt:   group.CreatedAt.Unix(),
		MemberCount: group.MemberCount,
	}
}

func (s *UserService) publishGroupInvite(ctx context.Context, groupID, groupName, inviterID, inviteeEmail string) {
	inviter, err := s.userRepo.GetUserByID(ctx, inviterID)
	if err != nil {
		log.Printf("Failed to get inviter: %v", err)
		return
	}

	inviterName := inviter.FirstName + " " + inviter.LastName
	if inviterName == " " {
		inviterName = inviter.Email
	}

	event := map[string]string{
		"group_id":      groupID,
		"group_name":    groupName,
		"inviter_name":  inviterName,
		"invitee_email": inviteeEmail,
	}
	eventData, _ := json.Marshal(event)

	if err := s.rabbitMQ.Publish(ctx, "user.group_invites", eventData); err != nil {
		log.Printf("Failed to publish group_invite event: %v", err)
	}
}

func (s *UserService) DeleteAvatar(ctx context.Context, req *pb.DeleteAvatarRequest) (*pb.DeleteAvatarResponse, error) {
	user, err := s.userRepo.GetUserByID(ctx, req.UserId)
	if err != nil {
		return nil, errors.New(codes.NotFound, errors.ReasonUserNotFound, "User not found", map[string]string{"user_id": req.UserId})
	}

	if user.AvatarURL == "" {
		return nil, errors.New(codes.FailedPrecondition, errors.ReasonNoAvatar, "User has no avatar", map[string]string{"user_id": req.UserId})
	}

	if err := s.deleteAvatarFile(ctx, user.AvatarURL); err != nil {
		log.Printf("Failed to delete avatar file: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonAvatarDeleteFailed, "Failed to delete avatar", map[string]string{"user_id": req.UserId})
	}

	user.AvatarURL = ""
	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		log.Printf("Failed to update user: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonUserUpdateFailed, "Failed to update profile", map[string]string{"user_id": req.UserId})
	}

	return &pb.DeleteAvatarResponse{}, nil
}

func (s *UserService) uploadAvatar(ctx context.Context, userID, filename string, data []byte, oldAvatarURL string) (string, error) {
	if oldAvatarURL != "" {
		if err := s.deleteAvatarFile(ctx, oldAvatarURL); err != nil {
			log.Printf("Failed to delete old avatar (continuing with upload): %v", err)
		}
	}

	objectName := userID + "/" + filename

	contentType := "application/octet-stream"
	ext := filepath.Ext(filename)
	switch ext {
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	}

	reader := bytes.NewReader(data)
	err := s.s3Client.UploadFile(ctx, constants.AvatarBucketName, objectName, reader, int64(len(data)), contentType)
	if err != nil {
		return "", err
	}

	return s.s3Client.GetPublicURL(constants.AvatarBucketName, objectName), nil
}

func (s *UserService) deleteAvatarFile(ctx context.Context, avatarURL string) error {
	bucketPrefix := "/" + constants.AvatarBucketName + "/"
	_, after, ok := strings.Cut(avatarURL, bucketPrefix)
	if !ok {
		return nil
	}
	objectName := after

	return s.s3Client.DeleteFile(ctx, constants.AvatarBucketName, objectName)
}
