package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"path/filepath"

	"user-service/internal/repository"
	"user-service/pkg/messaging"
	"user-service/pkg/storage"
	pb "user-service/proto"
)

type UserService struct {
	pb.UnimplementedUserServiceServer
	userRepo     *repository.UserRepository
	settingsRepo *repository.NotificationSettingsRepository
	groupRepo    *repository.GroupRepository
	s3Client     *storage.S3Client
	rabbitMQ     *messaging.RabbitMQClient
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

func (s *UserService) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	log.Printf("Register user %s", req.UserId)

	user, err := s.userRepo.GetUserByID(ctx, req.UserId)
	if err != nil {
		log.Printf("Failed to get user by ID %s: %v", req.UserId, err)
		return &pb.RegisterResponse{
			Success: false,
			Message: "User not found",
		}, nil
	}

	log.Printf("Found user: %s (email: %s, is_registered: %v)", user.ID, user.Email, user.IsRegistered)

	user.FirstName = req.FirstName
	user.LastName = req.LastName
	user.IsRegistered = true

	if len(req.AvatarData) > 0 && req.AvatarFilename != "" {
		avatarURL, err := s.uploadAvatar(ctx, req.UserId, req.AvatarFilename, req.AvatarData)
		if err != nil {
			log.Printf("Failed to upload avatar: %v", err)
			return &pb.RegisterResponse{
				Success: false,
				Message: "Failed to upload avatar",
			}, nil
		}
		user.AvatarURL = avatarURL
	}

	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		log.Printf("Failed to update user: %v", err)
		return &pb.RegisterResponse{
			Success: false,
			Message: "Failed to update profile",
		}, nil
	}

	log.Printf("User %s registered successfully", user.ID)

	_, err = s.settingsRepo.CreateDefaultSettings(ctx, user.ID)
	if err != nil {
		log.Printf("Failed to create notification settings: %v", err)
	}

	return &pb.RegisterResponse{
		Success: true,
		User:    s.userToProto(user),
		Message: "Registration completed successfully",
	}, nil
}

func (s *UserService) GetProfile(ctx context.Context, req *pb.GetProfileRequest) (*pb.GetProfileResponse, error) {
	user, err := s.userRepo.GetUserByID(ctx, req.UserId)
	if err != nil {
		return &pb.GetProfileResponse{
			Success: false,
			Message: "User not found",
		}, nil
	}

	return &pb.GetProfileResponse{
		Success: true,
		User:    s.userToProto(user),
		Message: "Profile retrieved successfully",
	}, nil
}

func (s *UserService) GetProfileByEmail(ctx context.Context, req *pb.GetProfileByEmailRequest) (*pb.GetProfileByEmailResponse, error) {
	user, err := s.userRepo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return &pb.GetProfileByEmailResponse{
			Success: false,
			Message: "User not found",
		}, nil
	}

	return &pb.GetProfileByEmailResponse{
		Success: true,
		User:    s.userToProto(user),
		Message: "Profile retrieved successfully",
	}, nil
}

func (s *UserService) UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest) (*pb.UpdateProfileResponse, error) {
	user, err := s.userRepo.GetUserByID(ctx, req.UserId)
	if err != nil {
		return &pb.UpdateProfileResponse{
			Success: false,
			Message: "User not found",
		}, nil
	}

	if req.FirstName != "" {
		user.FirstName = req.FirstName
	}
	if req.LastName != "" {
		user.LastName = req.LastName
	}

	if len(req.AvatarData) > 0 && req.AvatarFilename != "" {
		avatarURL, err := s.uploadAvatar(ctx, req.UserId, req.AvatarFilename, req.AvatarData)
		if err != nil {
			log.Printf("Failed to upload avatar: %v", err)
			return &pb.UpdateProfileResponse{
				Success: false,
				Message: "Failed to upload avatar",
			}, nil
		}
		user.AvatarURL = avatarURL
	}

	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		log.Printf("Failed to update user: %v", err)
		return &pb.UpdateProfileResponse{
			Success: false,
			Message: "Failed to update profile",
		}, nil
	}

	return &pb.UpdateProfileResponse{
		Success: true,
		User:    s.userToProto(user),
		Message: "Profile updated successfully",
	}, nil
}

func (s *UserService) GetNotificationSettings(ctx context.Context, req *pb.GetNotificationSettingsRequest) (*pb.GetNotificationSettingsResponse, error) {
	settings, err := s.settingsRepo.GetOrCreateSettings(ctx, req.UserId)
	if err != nil {
		log.Printf("Failed to get notification settings: %v", err)
		return &pb.GetNotificationSettingsResponse{
			Success: false,
			Message: "Failed to retrieve settings",
		}, nil
	}

	return &pb.GetNotificationSettingsResponse{
		Success:  true,
		Settings: s.settingsToProto(settings),
		Message:  "Settings retrieved successfully",
	}, nil
}

func (s *UserService) UpdateNotificationSettings(ctx context.Context, req *pb.UpdateNotificationSettingsRequest) (*pb.UpdateNotificationSettingsResponse, error) {
	log.Printf("UpdateNotificationSettings for user %s: NewQuizzes=%v, QuizResults=%v, GroupInvites=%v, DeadlineReminder=%v",
		req.UserId, req.NewQuizzes, req.QuizResults, req.GroupInvites, req.DeadlineReminder)

	settings, err := s.settingsRepo.GetOrCreateSettings(ctx, req.UserId)
	if err != nil {
		log.Printf("Failed to get notification settings: %v", err)
		return &pb.UpdateNotificationSettingsResponse{
			Success: false,
			Message: "Failed to retrieve settings",
		}, nil
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
		return &pb.UpdateNotificationSettingsResponse{
			Success: false,
			Message: "Failed to update settings",
		}, nil
	}

	log.Printf("Settings saved successfully")

	return &pb.UpdateNotificationSettingsResponse{
		Success:  true,
		Settings: s.settingsToProto(settings),
		Message:  "Settings updated successfully",
	}, nil
}

func (s *UserService) CreateGroup(ctx context.Context, req *pb.CreateGroupRequest) (*pb.CreateGroupResponse, error) {
	group, err := s.groupRepo.CreateGroup(ctx, req.Name, req.OwnerId)
	if err != nil {
		log.Printf("Failed to create group: %v", err)
		return &pb.CreateGroupResponse{
			Success: false,
			Message: "Failed to create group",
		}, nil
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
		Success: true,
		Group:   s.groupToProto(group),
		Message: "Group created successfully",
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
		return &pb.GetGroupsResponse{
			Success: false,
			Message: "Invalid filter",
		}, nil
	}

	if err != nil {
		log.Printf("Failed to get groups: %v", err)
		return &pb.GetGroupsResponse{
			Success: false,
			Message: "Failed to retrieve groups",
		}, nil
	}

	protoGroups := make([]*pb.Group, len(groups))
	for i, group := range groups {
		protoGroups[i] = s.groupToProto(group)
	}

	return &pb.GetGroupsResponse{
		Success: true,
		Groups:  protoGroups,
		Message: "Groups retrieved successfully",
	}, nil
}

func (s *UserService) GetGroup(ctx context.Context, req *pb.GetGroupRequest) (*pb.GetGroupResponse, error) {
	group, err := s.groupRepo.GetGroupByID(ctx, req.GroupId)
	if err != nil {
		return &pb.GetGroupResponse{
			Success: false,
			Message: "Group not found",
		}, nil
	}

	isMember, err := s.groupRepo.IsMember(ctx, req.GroupId, req.UserId)
	if err != nil || (!isMember && group.OwnerID != req.UserId) {
		return &pb.GetGroupResponse{
			Success: false,
			Message: "Access denied",
		}, nil
	}

	users, err := s.groupRepo.GetGroupUsers(ctx, req.GroupId)
	if err != nil {
		log.Printf("Failed to get group users: %v", err)
		return &pb.GetGroupResponse{
			Success: false,
			Message: "Failed to retrieve members",
		}, nil
	}

	members := make([]*pb.User, 0, len(users))
	for _, user := range users {
		members = append(members, s.userToProto(user))
	}

	return &pb.GetGroupResponse{
		Success: true,
		Group: &pb.GroupWithMembers{
			Group:   s.groupToProto(group),
			Members: members,
		},
		Message: "Group retrieved successfully",
	}, nil
}

func (s *UserService) UpdateGroup(ctx context.Context, req *pb.UpdateGroupRequest) (*pb.UpdateGroupResponse, error) {
	group, err := s.groupRepo.GetGroupByID(ctx, req.GroupId)
	if err != nil {
		return &pb.UpdateGroupResponse{
			Success: false,
			Message: "Group not found",
		}, nil
	}

	if group.OwnerID != req.UserId {
		return &pb.UpdateGroupResponse{
			Success: false,
			Message: "Only group owner can update group",
		}, nil
	}

	if req.Name != "" {
		group.Name = req.Name
		if err := s.groupRepo.UpdateGroup(ctx, group); err != nil {
			log.Printf("Failed to update group: %v", err)
			return &pb.UpdateGroupResponse{
				Success: false,
				Message: "Failed to update group",
			}, nil
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
		Success: true,
		Group:   s.groupToProto(group),
		Message: "Group updated successfully",
	}, nil
}

func (s *UserService) DeleteGroup(ctx context.Context, req *pb.DeleteGroupRequest) (*pb.DeleteGroupResponse, error) {
	group, err := s.groupRepo.GetGroupByID(ctx, req.GroupId)
	if err != nil {
		return &pb.DeleteGroupResponse{
			Success: false,
			Message: "Group not found",
		}, nil
	}

	if group.OwnerID != req.UserId {
		return &pb.DeleteGroupResponse{
			Success: false,
			Message: "Only group owner can delete group",
		}, nil
	}

	if err := s.groupRepo.DeleteGroup(ctx, req.GroupId); err != nil {
		log.Printf("Failed to delete group: %v", err)
		return &pb.DeleteGroupResponse{
			Success: false,
			Message: "Failed to delete group",
		}, nil
	}

	return &pb.DeleteGroupResponse{
		Success: true,
		Message: "Group deleted successfully",
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

func (s *UserService) uploadAvatar(ctx context.Context, userID, filename string, data []byte) (string, error) {
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
	err := s.s3Client.UploadFile(ctx, "user-avatars", objectName, reader, int64(len(data)), contentType)
	if err != nil {
		return "", err
	}

	return "/avatars/" + objectName, nil
}
