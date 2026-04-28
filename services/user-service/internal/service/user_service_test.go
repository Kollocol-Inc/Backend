package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"user-service/internal/repository"
	"user-service/internal/service/mocks"
	pb "user-service/proto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type testDeps struct {
	svc                *UserService
	userRepo           *mocks.MockUserRepo
	settingsRepo       *mocks.MockSettingsRepo
	groupRepo          *mocks.MockGroupRepo
	s3                 *mocks.MockFileStorage
	publisher          *mocks.MockMessagePublisher
	authClient         *mocks.MockAuthServiceClient
	quizClient         *mocks.MockQuizServiceClient
	notificationClient *mocks.MockNotificationServiceClient
}

func setupTest(t *testing.T) *testDeps {
	ctrl := gomock.NewController(t)
	userRepo := mocks.NewMockUserRepo(ctrl)
	settingsRepo := mocks.NewMockSettingsRepo(ctrl)
	groupRepo := mocks.NewMockGroupRepo(ctrl)
	s3 := mocks.NewMockFileStorage(ctrl)
	publisher := mocks.NewMockMessagePublisher(ctrl)
	authClient := mocks.NewMockAuthServiceClient(ctrl)
	quizClient := mocks.NewMockQuizServiceClient(ctrl)
	notificationClient := mocks.NewMockNotificationServiceClient(ctrl)

	svc := NewUserServiceWithDeps(userRepo, settingsRepo, groupRepo, s3, publisher, authClient, quizClient, notificationClient)
	return &testDeps{
		svc:                svc,
		userRepo:           userRepo,
		settingsRepo:       settingsRepo,
		groupRepo:          groupRepo,
		s3:                 s3,
		publisher:          publisher,
		authClient:         authClient,
		quizClient:         quizClient,
		notificationClient: notificationClient,
	}
}

func testUser() *repository.User {
	return &repository.User{
		ID: "user-1", Email: "test@example.com",
		FirstName: "John", LastName: "Doe",
		AvatarURL: "", IsRegistered: false,
		CreatedAt: time.Now(),
	}
}

func TestRegister_Success(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	user := testUser()
	d.userRepo.EXPECT().GetUserByID(ctx, "user-1").Return(user, nil)
	d.userRepo.EXPECT().UpdateUser(ctx, gomock.Any()).Return(nil)
	d.settingsRepo.EXPECT().CreateDefaultSettings(ctx, "user-1").Return(nil, nil)

	resp, err := d.svc.Register(ctx, &pb.RegisterRequest{
		UserId:    "user-1",
		FirstName: "John",
		LastName:  "Doe",
	})

	require.NoError(t, err)
	assert.Equal(t, "John", resp.User.FirstName)
	assert.True(t, resp.User.IsRegistered)
}

func TestRegister_UserNotFound(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	d.userRepo.EXPECT().GetUserByID(ctx, "user-999").Return(nil, fmt.Errorf("not found"))

	_, err := d.svc.Register(ctx, &pb.RegisterRequest{UserId: "user-999"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "User not found")
}

func TestRegister_WithAvatar(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	user := testUser()
	d.userRepo.EXPECT().GetUserByID(ctx, "user-1").Return(user, nil)
	d.s3.EXPECT().UploadFile(ctx, "user-avatars", "user-1/photo.png", gomock.Any(), int64(4), "image/png").Return(nil)
	d.s3.EXPECT().GetPublicURL("user-avatars", "user-1/photo.png").Return("http://s3/user-avatars/user-1/photo.png")
	d.userRepo.EXPECT().UpdateUser(ctx, gomock.Any()).Return(nil)
	d.settingsRepo.EXPECT().CreateDefaultSettings(ctx, "user-1").Return(nil, nil)

	resp, err := d.svc.Register(ctx, &pb.RegisterRequest{
		UserId:         "user-1",
		FirstName:      "John",
		LastName:       "Doe",
		AvatarData:     []byte("test"),
		AvatarFilename: "photo.png",
	})

	require.NoError(t, err)
	assert.Equal(t, "http://s3/user-avatars/user-1/photo.png", resp.User.AvatarUrl)
}

func TestGetProfile_Success(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	user := testUser()
	user.IsRegistered = true
	d.userRepo.EXPECT().GetUserByID(ctx, "user-1").Return(user, nil)

	resp, err := d.svc.GetProfile(ctx, &pb.GetProfileRequest{UserId: "user-1"})
	require.NoError(t, err)
	assert.Equal(t, "user-1", resp.User.Id)
}

func TestGetProfile_NotFound(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	d.userRepo.EXPECT().GetUserByID(ctx, "user-999").Return(nil, fmt.Errorf("not found"))

	_, err := d.svc.GetProfile(ctx, &pb.GetProfileRequest{UserId: "user-999"})
	assert.Error(t, err)
}

func TestUpdateProfile_PartialUpdate(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	user := testUser()
	user.FirstName = "Old"
	user.LastName = "Name"
	d.userRepo.EXPECT().GetUserByID(ctx, "user-1").Return(user, nil)
	d.userRepo.EXPECT().UpdateUser(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, u *repository.User) error {
		assert.Equal(t, "New", u.FirstName)
		assert.Equal(t, "Name", u.LastName) // unchanged
		return nil
	})

	resp, err := d.svc.UpdateProfile(ctx, &pb.UpdateProfileRequest{
		UserId:    "user-1",
		FirstName: "New",
	})
	require.NoError(t, err)
	assert.Equal(t, "New", resp.User.FirstName)
}

func TestGetNotificationSettings_Success(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	settings := &repository.NotificationSettings{
		UserID: "user-1", NewQuizzes: true, QuizResults: true,
		GroupInvites: true, DeadlineReminder: "24h", UpdatedAt: time.Now(),
	}
	d.settingsRepo.EXPECT().GetOrCreateSettings(ctx, "user-1").Return(settings, nil)

	resp, err := d.svc.GetNotificationSettings(ctx, &pb.GetNotificationSettingsRequest{UserId: "user-1"})
	require.NoError(t, err)
	assert.True(t, resp.Settings.NewQuizzes)
	assert.Equal(t, "24h", resp.Settings.DeadlineReminder)
}

func TestUpdateNotificationSettings_PartialUpdate(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	settings := &repository.NotificationSettings{
		UserID: "user-1", NewQuizzes: true, QuizResults: true,
		GroupInvites: true, DeadlineReminder: "24h", UpdatedAt: time.Now(),
	}
	d.settingsRepo.EXPECT().GetOrCreateSettings(ctx, "user-1").Return(settings, nil)
	d.settingsRepo.EXPECT().UpdateSettings(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, s *repository.NotificationSettings) error {
		assert.False(t, s.NewQuizzes)
		assert.True(t, s.QuizResults) // unchanged
		return nil
	})

	newQuizzes := false
	resp, err := d.svc.UpdateNotificationSettings(ctx, &pb.UpdateNotificationSettingsRequest{
		UserId:     "user-1",
		NewQuizzes: &newQuizzes,
	})
	require.NoError(t, err)
	assert.False(t, resp.Settings.NewQuizzes)
}

func TestCreateGroup_Success(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	group := &repository.Group{ID: "grp-1", Name: "Test Group", OwnerID: "user-1", CreatedAt: time.Now()}
	usersMap := map[string]*repository.User{
		"member@example.com": {ID: "user-2", Email: "member@example.com", FirstName: "Jane", LastName: "Doe"},
	}
	inviter := &repository.User{ID: "user-1", Email: "owner@example.com", FirstName: "John", LastName: "Doe"}

	d.groupRepo.EXPECT().CreateGroup(ctx, "Test Group", "user-1").Return(group, nil)
	d.userRepo.EXPECT().GetUsersByEmailsMap(ctx, []string{"member@example.com"}).Return(usersMap, nil)
	d.groupRepo.EXPECT().AddMembers(ctx, "grp-1", []string{"user-1", "user-2"}).Return(nil)
	d.userRepo.EXPECT().GetUserByID(ctx, "user-1").Return(inviter, nil)
	d.userRepo.EXPECT().GetUserByEmail(ctx, "member@example.com").Return(&repository.User{ID: "user-2", Email: "member@example.com", IsRegistered: false}, nil)
	d.publisher.EXPECT().Publish(ctx, "user.group_invites", gomock.Any()).Return(nil)
	d.groupRepo.EXPECT().GetMemberCount(ctx, "grp-1").Return(int32(2), nil)

	resp, err := d.svc.CreateGroup(ctx, &pb.CreateGroupRequest{
		Name:         "Test Group",
		OwnerId:      "user-1",
		MemberEmails: []string{"member@example.com"},
	})

	require.NoError(t, err)
	assert.Equal(t, "Test Group", resp.Group.Name)
	assert.Equal(t, int32(2), resp.Group.MemberCount)
}

func TestGetGroups_Created(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	groups := []*repository.Group{
		{ID: "grp-1", Name: "Group 1", OwnerID: "user-1", CreatedAt: time.Now(), MemberCount: 3},
	}
	d.groupRepo.EXPECT().GetCreatedGroups(ctx, "user-1").Return(groups, nil)

	resp, err := d.svc.GetGroups(ctx, &pb.GetGroupsRequest{UserId: "user-1", Filter: "created"})
	require.NoError(t, err)
	assert.Len(t, resp.Groups, 1)
}

func TestGetGroups_InvalidFilter(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	_, err := d.svc.GetGroups(ctx, &pb.GetGroupsRequest{UserId: "user-1", Filter: "invalid"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid filter")
}

func TestGetGroup_AsMember(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	group := &repository.Group{ID: "grp-1", Name: "Group", OwnerID: "user-1", MemberCount: 2}
	users := []*repository.User{{ID: "user-1"}, {ID: "user-2"}}

	d.groupRepo.EXPECT().GetGroupByID(ctx, "grp-1").Return(group, nil)
	d.groupRepo.EXPECT().IsMember(ctx, "grp-1", "user-2").Return(true, nil)
	d.groupRepo.EXPECT().GetGroupUsers(ctx, "grp-1").Return(users, nil)

	resp, err := d.svc.GetGroup(ctx, &pb.GetGroupRequest{GroupId: "grp-1", UserId: "user-2"})
	require.NoError(t, err)
	assert.Len(t, resp.Group.Members, 2)
}

func TestGetGroup_AccessDenied(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	group := &repository.Group{ID: "grp-1", Name: "Group", OwnerID: "user-1"}
	d.groupRepo.EXPECT().GetGroupByID(ctx, "grp-1").Return(group, nil)
	d.groupRepo.EXPECT().IsMember(ctx, "grp-1", "user-3").Return(false, nil)

	_, err := d.svc.GetGroup(ctx, &pb.GetGroupRequest{GroupId: "grp-1", UserId: "user-3"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Access denied")
}

func TestDeleteGroup_Success(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	group := &repository.Group{ID: "grp-1", OwnerID: "user-1"}
	d.groupRepo.EXPECT().GetGroupByID(ctx, "grp-1").Return(group, nil)
	d.groupRepo.EXPECT().DeleteGroup(ctx, "grp-1").Return(nil)

	_, err := d.svc.DeleteGroup(ctx, &pb.DeleteGroupRequest{GroupId: "grp-1", UserId: "user-1"})
	require.NoError(t, err)
}

func TestDeleteGroup_NotOwner(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	group := &repository.Group{ID: "grp-1", OwnerID: "user-1"}
	d.groupRepo.EXPECT().GetGroupByID(ctx, "grp-1").Return(group, nil)

	_, err := d.svc.DeleteGroup(ctx, &pb.DeleteGroupRequest{GroupId: "grp-1", UserId: "user-2"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Only group owner")
}

func TestCheckGroupMembership_Owner(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	group := &repository.Group{ID: "grp-1", OwnerID: "user-1"}
	d.groupRepo.EXPECT().IsMember(ctx, "grp-1", "user-1").Return(true, nil)
	d.groupRepo.EXPECT().GetGroupByID(ctx, "grp-1").Return(group, nil)

	resp, err := d.svc.CheckGroupMembership(ctx, &pb.CheckGroupMembershipRequest{GroupId: "grp-1", UserId: "user-1"})
	require.NoError(t, err)
	assert.True(t, resp.IsMember)
	assert.Equal(t, "owner", resp.Role)
}

func TestCheckGroupMembership_NotMember(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	d.groupRepo.EXPECT().IsMember(ctx, "grp-1", "user-3").Return(false, nil)

	resp, err := d.svc.CheckGroupMembership(ctx, &pb.CheckGroupMembershipRequest{GroupId: "grp-1", UserId: "user-3"})
	require.NoError(t, err)
	assert.False(t, resp.IsMember)
}

func TestDeleteAvatar_Success(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	user := testUser()
	user.AvatarURL = "http://s3/user-avatars/user-1/photo.png"
	d.userRepo.EXPECT().GetUserByID(ctx, "user-1").Return(user, nil)
	d.s3.EXPECT().DeleteFile(ctx, "user-avatars", "user-1/photo.png").Return(nil)
	d.userRepo.EXPECT().UpdateUser(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, u *repository.User) error {
		assert.Empty(t, u.AvatarURL)
		return nil
	})

	_, err := d.svc.DeleteAvatar(ctx, &pb.DeleteAvatarRequest{UserId: "user-1"})
	require.NoError(t, err)
}

func TestDeleteAvatar_NoAvatar(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	user := testUser()
	user.AvatarURL = ""
	d.userRepo.EXPECT().GetUserByID(ctx, "user-1").Return(user, nil)

	_, err := d.svc.DeleteAvatar(ctx, &pb.DeleteAvatarRequest{UserId: "user-1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no avatar")
}

func TestDeleteUser_Success(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	user := testUser()
	user.AvatarURL = "http://localhost:9000/user-avatars/user-1/avatar.jpg"

	gomock.InOrder(
		d.userRepo.EXPECT().GetUserByID(ctx, "user-1").Return(user, nil),
		d.authClient.EXPECT().RevokeUser(ctx, "user-1").Return(nil),
		d.quizClient.EXPECT().DeleteAllByOwner(ctx, "user-1").Return(nil),
		d.notificationClient.EXPECT().DeleteAllForUser(ctx, "user-1").Return(nil),
		d.groupRepo.EXPECT().DeleteOwnedGroups(ctx, "user-1").Return(nil),
		d.groupRepo.EXPECT().DeleteUserMemberships(ctx, "user-1").Return(nil),
		d.settingsRepo.EXPECT().DeleteSettings(ctx, "user-1").Return(nil),
		d.s3.EXPECT().DeleteFile(ctx, "user-avatars", "user-1/avatar.jpg").Return(nil),
		d.userRepo.EXPECT().DeleteUser(ctx, "user-1").Return(nil),
	)

	resp, err := d.svc.DeleteUser(ctx, &pb.DeleteUserRequest{UserId: "user-1"})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestDeleteUser_SuccessNoAvatar(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	user := testUser() // AvatarURL is empty — s3.DeleteFile must NOT be called

	gomock.InOrder(
		d.userRepo.EXPECT().GetUserByID(ctx, "user-1").Return(user, nil),
		d.authClient.EXPECT().RevokeUser(ctx, "user-1").Return(nil),
		d.quizClient.EXPECT().DeleteAllByOwner(ctx, "user-1").Return(nil),
		d.notificationClient.EXPECT().DeleteAllForUser(ctx, "user-1").Return(nil),
		d.groupRepo.EXPECT().DeleteOwnedGroups(ctx, "user-1").Return(nil),
		d.groupRepo.EXPECT().DeleteUserMemberships(ctx, "user-1").Return(nil),
		d.settingsRepo.EXPECT().DeleteSettings(ctx, "user-1").Return(nil),
		d.userRepo.EXPECT().DeleteUser(ctx, "user-1").Return(nil),
	)

	_, err := d.svc.DeleteUser(ctx, &pb.DeleteUserRequest{UserId: "user-1"})
	require.NoError(t, err)
}

func TestDeleteUser_EmptyUserID(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	_, err := d.svc.DeleteUser(ctx, &pb.DeleteUserRequest{UserId: ""})
	assert.Error(t, err)
}

func TestDeleteUser_UserNotFound(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	d.userRepo.EXPECT().GetUserByID(ctx, "user-1").Return(nil, fmt.Errorf("not found"))

	_, err := d.svc.DeleteUser(ctx, &pb.DeleteUserRequest{UserId: "user-1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "User not found")
}

func TestDeleteUser_RevokeFails_NoOtherCalls(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	d.userRepo.EXPECT().GetUserByID(ctx, "user-1").Return(testUser(), nil)
	d.authClient.EXPECT().RevokeUser(ctx, "user-1").Return(fmt.Errorf("auth down"))
	// no other mocks expect calls — if they're invoked, gomock fails the test

	_, err := d.svc.DeleteUser(ctx, &pb.DeleteUserRequest{UserId: "user-1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to revoke")
}

func TestDeleteUser_QuizFails_StopsAfterRevoke(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	d.userRepo.EXPECT().GetUserByID(ctx, "user-1").Return(testUser(), nil)
	d.authClient.EXPECT().RevokeUser(ctx, "user-1").Return(nil)
	d.quizClient.EXPECT().DeleteAllByOwner(ctx, "user-1").Return(fmt.Errorf("quiz down"))

	_, err := d.svc.DeleteUser(ctx, &pb.DeleteUserRequest{UserId: "user-1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to delete quiz data")
}

func TestDeleteUser_NotificationsFail(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	d.userRepo.EXPECT().GetUserByID(ctx, "user-1").Return(testUser(), nil)
	d.authClient.EXPECT().RevokeUser(ctx, "user-1").Return(nil)
	d.quizClient.EXPECT().DeleteAllByOwner(ctx, "user-1").Return(nil)
	d.notificationClient.EXPECT().DeleteAllForUser(ctx, "user-1").Return(fmt.Errorf("notif down"))

	_, err := d.svc.DeleteUser(ctx, &pb.DeleteUserRequest{UserId: "user-1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to delete notifications")
}

func TestDeleteUser_OwnedGroupsFails(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	d.userRepo.EXPECT().GetUserByID(ctx, "user-1").Return(testUser(), nil)
	d.authClient.EXPECT().RevokeUser(ctx, "user-1").Return(nil)
	d.quizClient.EXPECT().DeleteAllByOwner(ctx, "user-1").Return(nil)
	d.notificationClient.EXPECT().DeleteAllForUser(ctx, "user-1").Return(nil)
	d.groupRepo.EXPECT().DeleteOwnedGroups(ctx, "user-1").Return(fmt.Errorf("db error"))

	_, err := d.svc.DeleteUser(ctx, &pb.DeleteUserRequest{UserId: "user-1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to delete owned groups")
}

func TestDeleteUser_AvatarFails_StopsBeforeUserRowDelete(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	user := testUser()
	user.AvatarURL = "http://localhost:9000/user-avatars/user-1/avatar.jpg"

	d.userRepo.EXPECT().GetUserByID(ctx, "user-1").Return(user, nil)
	d.authClient.EXPECT().RevokeUser(ctx, "user-1").Return(nil)
	d.quizClient.EXPECT().DeleteAllByOwner(ctx, "user-1").Return(nil)
	d.notificationClient.EXPECT().DeleteAllForUser(ctx, "user-1").Return(nil)
	d.groupRepo.EXPECT().DeleteOwnedGroups(ctx, "user-1").Return(nil)
	d.groupRepo.EXPECT().DeleteUserMemberships(ctx, "user-1").Return(nil)
	d.settingsRepo.EXPECT().DeleteSettings(ctx, "user-1").Return(nil)
	d.s3.EXPECT().DeleteFile(ctx, "user-avatars", "user-1/avatar.jpg").Return(fmt.Errorf("minio down"))
	// userRepo.DeleteUser is NOT called

	_, err := d.svc.DeleteUser(ctx, &pb.DeleteUserRequest{UserId: "user-1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to delete avatar")
}

func TestDeleteUser_UserRowDeleteFails(t *testing.T) {
	d := setupTest(t)
	ctx := context.Background()

	d.userRepo.EXPECT().GetUserByID(ctx, "user-1").Return(testUser(), nil)
	d.authClient.EXPECT().RevokeUser(ctx, "user-1").Return(nil)
	d.quizClient.EXPECT().DeleteAllByOwner(ctx, "user-1").Return(nil)
	d.notificationClient.EXPECT().DeleteAllForUser(ctx, "user-1").Return(nil)
	d.groupRepo.EXPECT().DeleteOwnedGroups(ctx, "user-1").Return(nil)
	d.groupRepo.EXPECT().DeleteUserMemberships(ctx, "user-1").Return(nil)
	d.settingsRepo.EXPECT().DeleteSettings(ctx, "user-1").Return(nil)
	d.userRepo.EXPECT().DeleteUser(ctx, "user-1").Return(fmt.Errorf("db error"))

	_, err := d.svc.DeleteUser(ctx, &pb.DeleteUserRequest{UserId: "user-1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to delete user")
}
