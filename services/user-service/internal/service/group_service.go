package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"

	"user-service/constants"
	"user-service/internal/repository"
	"user-service/pkg/errors"
	"user-service/pkg/lang"
	pb "user-service/proto"
)

const (
	groupInviteNotificationType = "group_invite"
	groupKickedNotificationType = "group_kicked"
)

func (s *UserService) CreateGroup(ctx context.Context, req *pb.CreateGroupRequest) (*pb.CreateGroupResponse, error) {
	if err := s.validateGroupAvatarURL(req.AvatarUrl); err != nil {
		return nil, err
	}

	usersMap, err := s.userRepo.GetUsersByEmailsMap(ctx, req.MemberEmails)
	if err != nil {
		log.Printf("CreateGroup: failed to lookup users by emails: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonGroupCreateFailed, "Failed to resolve member emails", map[string]string{"owner_id": req.OwnerId})
	}

	var (
		group         *repository.Group
		invitedEmails []string
	)
	err = s.txMgr.InTransaction(ctx, func(ctx context.Context) error {
		var err error
		group, err = s.groupRepo.CreateGroup(ctx, req.Name, req.Description, req.AvatarUrl, req.OwnerId)
		if err != nil {
			return fmt.Errorf("create group: %w", err)
		}

		if err := s.groupRepo.AddMembers(ctx, group.ID, []string{req.OwnerId}); err != nil {
			return fmt.Errorf("add owner as member: %w", err)
		}

		for _, email := range req.MemberEmails {
			user, exists := usersMap[email]
			if !exists {
				log.Printf("CreateGroup: skipping unregistered email %s", email)
				continue
			}
			if user.ID == req.OwnerId {
				continue
			}
			if err := s.groupRepo.AddInvitation(ctx, group.ID, user.ID, req.OwnerId); err != nil {
				return fmt.Errorf("add invitation: %w", err)
			}
			invitedEmails = append(invitedEmails, email)
		}
		return nil
	})
	if err != nil {
		log.Printf("CreateGroup tx failed: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonGroupCreateFailed, "Failed to create group", map[string]string{"owner_id": req.OwnerId})
	}

	for _, email := range invitedEmails {
		s.publishGroupInvite(ctx, group.ID, group.Name, req.OwnerId, email)
	}

	if refreshed, err := s.groupRepo.GetGroupByID(ctx, group.ID); err == nil {
		group = refreshed
	}

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
	case "invitee":
		groups, err = s.groupRepo.GetInvitedGroups(ctx, req.UserId)
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
	if err != nil {
		log.Printf("Failed to check membership: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonAccessDenied, "Failed to verify membership", map[string]string{"group_id": req.GroupId, "user_id": req.UserId})
	}

	isInvited := false
	if !isMember && group.OwnerID != req.UserId {
		isInvited, err = s.groupRepo.IsInvited(ctx, req.GroupId, req.UserId)
		if err != nil {
			log.Printf("Failed to check invitation: %v", err)
			return nil, errors.New(codes.Internal, errors.ReasonAccessDenied, "Failed to verify access", map[string]string{"group_id": req.GroupId, "user_id": req.UserId})
		}
	}

	if !isMember && !isInvited && group.OwnerID != req.UserId {
		return nil, errors.New(codes.PermissionDenied, errors.ReasonAccessDenied, "Access denied", map[string]string{"group_id": req.GroupId, "user_id": req.UserId})
	}

	memberUsers, err := s.groupRepo.GetGroupUsers(ctx, req.GroupId)
	if err != nil {
		log.Printf("Failed to get group members: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonMembersRetrieveFailed, "Failed to retrieve members", map[string]string{"group_id": req.GroupId})
	}

	invitedUsers, err := s.groupRepo.GetGroupInvitedUsers(ctx, req.GroupId)
	if err != nil {
		log.Printf("Failed to get invited users: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonMembersRetrieveFailed, "Failed to retrieve invited users", map[string]string{"group_id": req.GroupId})
	}

	members := make([]*pb.User, 0, len(memberUsers))
	for _, u := range memberUsers {
		members = append(members, s.userToProto(u))
	}
	invited := make([]*pb.User, 0, len(invitedUsers))
	for _, u := range invitedUsers {
		invited = append(invited, s.userToProto(u))
	}

	return &pb.GetGroupResponse{
		Group: &pb.GroupWithMembers{
			Group:        s.groupToProto(group),
			Members:      members,
			InvitedUsers: invited,
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

	oldAvatarURL := group.AvatarURL
	avatarChanged := false

	if req.Name != nil {
		group.Name = *req.Name
	}
	if req.Description != nil {
		group.Description = *req.Description
	}
	if req.AvatarUrl != nil {
		if err := s.validateGroupAvatarURL(*req.AvatarUrl); err != nil {
			return nil, err
		}
		if *req.AvatarUrl != oldAvatarURL {
			group.AvatarURL = *req.AvatarUrl
			avatarChanged = true
		}
	}

	if err := s.groupRepo.UpdateGroup(ctx, group); err != nil {
		log.Printf("UpdateGroup failed: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonGroupUpdateFailed, "Failed to update group", map[string]string{"group_id": req.GroupId})
	}

	if avatarChanged && oldAvatarURL != "" {
		if err := s.deleteGroupAvatarFile(ctx, oldAvatarURL); err != nil {
			log.Printf("UpdateGroup: failed to delete old avatar file %q: %v", oldAvatarURL, err)
		}
	}

	if refreshed, err := s.groupRepo.GetGroupByID(ctx, group.ID); err == nil {
		group = refreshed
	}

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

	avatarURL := group.AvatarURL
	if err := s.groupRepo.DeleteGroup(ctx, req.GroupId); err != nil {
		log.Printf("Failed to delete group: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonGroupDeleteFailed, "Failed to delete group", map[string]string{"group_id": req.GroupId})
	}

	if avatarURL != "" {
		if err := s.deleteGroupAvatarFile(ctx, avatarURL); err != nil {
			log.Printf("DeleteGroup: failed to delete avatar file %q: %v", avatarURL, err)
		}
	}

	return &pb.DeleteGroupResponse{}, nil
}

func (s *UserService) CheckGroupMembership(ctx context.Context, req *pb.CheckGroupMembershipRequest) (*pb.CheckGroupMembershipResponse, error) {
	isMember, err := s.groupRepo.IsMember(ctx, req.GroupId, req.UserId)
	if err != nil {
		log.Printf("Failed to check group membership: %v", err)
		return &pb.CheckGroupMembershipResponse{IsMember: false, Role: ""}, nil
	}

	if !isMember {
		return &pb.CheckGroupMembershipResponse{IsMember: false, Role: ""}, nil
	}

	group, err := s.groupRepo.GetGroupByID(ctx, req.GroupId)
	if err != nil {
		log.Printf("Failed to get group: %v", err)
		return &pb.CheckGroupMembershipResponse{IsMember: true, Role: "member"}, nil
	}

	role := "member"
	if group.OwnerID == req.UserId {
		role = "owner"
	}
	return &pb.CheckGroupMembershipResponse{IsMember: true, Role: role}, nil
}

func (s *UserService) GetGroupMemberIDs(ctx context.Context, req *pb.GetGroupMemberIDsRequest) (*pb.GetGroupMemberIDsResponse, error) {
	ids, err := s.groupRepo.GetMemberIDs(ctx, req.GroupId)
	if err != nil {
		log.Printf("GetGroupMemberIDs: failed for group %s: %v", req.GroupId, err)
		return nil, errors.New(codes.Internal, errors.ReasonMembersRetrieveFailed, "Failed to get member IDs", map[string]string{"group_id": req.GroupId})
	}
	return &pb.GetGroupMemberIDsResponse{UserIds: ids}, nil
}

func (s *UserService) UploadGroupAvatar(ctx context.Context, req *pb.UploadGroupAvatarRequest) (*pb.UploadGroupAvatarResponse, error) {
	if len(req.AvatarData) == 0 || req.AvatarFilename == "" {
		return nil, errors.New(codes.InvalidArgument, errors.ReasonAvatarUploadFailed, "Avatar data and filename are required", nil)
	}

	objectName := uuid.New().String() + filepath.Ext(req.AvatarFilename)
	contentType := contentTypeForFilename(req.AvatarFilename)

	reader := bytes.NewReader(req.AvatarData)
	if err := s.s3Client.UploadFile(ctx, constants.GroupAvatarBucketName, objectName, reader, int64(len(req.AvatarData)), contentType); err != nil {
		log.Printf("UploadGroupAvatar: S3 upload failed: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonAvatarUploadFailed, "Failed to upload group avatar", nil)
	}

	url := s.s3Client.GetPublicURL(constants.GroupAvatarBucketName, objectName)
	return &pb.UploadGroupAvatarResponse{AvatarUrl: url}, nil
}

func (s *UserService) AcceptGroupInvite(ctx context.Context, req *pb.AcceptGroupInviteRequest) (*pb.AcceptGroupInviteResponse, error) {
	group, err := s.groupRepo.GetGroupByID(ctx, req.GroupId)
	if err != nil {
		return nil, errors.New(codes.NotFound, errors.ReasonGroupNotFound, "Group not found", map[string]string{"group_id": req.GroupId})
	}

	isMember, err := s.groupRepo.IsMember(ctx, req.GroupId, req.UserId)
	if err != nil {
		log.Printf("AcceptGroupInvite: failed to check membership: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonGroupUpdateFailed, "Failed to check membership", map[string]string{"group_id": req.GroupId})
	}
	if isMember {
		return nil, errors.New(codes.AlreadyExists, errors.ReasonAlreadyMember, "User is already a member", map[string]string{"group_id": req.GroupId, "user_id": req.UserId})
	}

	isInvited, err := s.groupRepo.IsInvited(ctx, req.GroupId, req.UserId)
	if err != nil {
		log.Printf("AcceptGroupInvite: failed to check invitation: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonGroupUpdateFailed, "Failed to check invitation", map[string]string{"group_id": req.GroupId})
	}
	if !isInvited {
		return nil, errors.New(codes.NotFound, errors.ReasonInvitationNotFound, "No pending invitation for this group", map[string]string{"group_id": req.GroupId, "user_id": req.UserId})
	}

	if err := s.txMgr.InTransaction(ctx, func(ctx context.Context) error {
		return s.groupRepo.AcceptInvitation(ctx, req.GroupId, req.UserId)
	}); err != nil {
		if strings.Contains(err.Error(), "invitation not found") {
			return nil, errors.New(codes.NotFound, errors.ReasonInvitationNotFound, "No pending invitation for this group", map[string]string{"group_id": req.GroupId, "user_id": req.UserId})
		}
		log.Printf("AcceptGroupInvite: tx failed: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonGroupUpdateFailed, "Failed to accept invitation", map[string]string{"group_id": req.GroupId})
	}

	if err := s.notificationClient.MarkAsReadByType(ctx, req.UserId, groupInviteNotificationType, req.GroupId); err != nil {
		log.Printf("AcceptGroupInvite: best-effort mark-read failed: %v", err)
	}

	if refreshed, err := s.groupRepo.GetGroupByID(ctx, group.ID); err == nil {
		group = refreshed
	}

	return &pb.AcceptGroupInviteResponse{
		Group: s.groupToProto(group),
	}, nil
}

func (s *UserService) DeclineGroupInvite(ctx context.Context, req *pb.DeclineGroupInviteRequest) (*pb.DeclineGroupInviteResponse, error) {
	if _, err := s.groupRepo.GetGroupByID(ctx, req.GroupId); err != nil {
		return nil, errors.New(codes.NotFound, errors.ReasonGroupNotFound, "Group not found", map[string]string{"group_id": req.GroupId})
	}

	isInvited, err := s.groupRepo.IsInvited(ctx, req.GroupId, req.UserId)
	if err != nil {
		log.Printf("DeclineGroupInvite: failed to check invitation: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonGroupUpdateFailed, "Failed to check invitation", map[string]string{"group_id": req.GroupId})
	}
	if !isInvited {
		return nil, errors.New(codes.NotFound, errors.ReasonInvitationNotFound, "No pending invitation for this group", map[string]string{"group_id": req.GroupId, "user_id": req.UserId})
	}

	if err := s.groupRepo.RemoveInvitation(ctx, req.GroupId, req.UserId); err != nil {
		log.Printf("DeclineGroupInvite: failed to remove invitation: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonGroupUpdateFailed, "Failed to decline invitation", map[string]string{"group_id": req.GroupId})
	}

	if err := s.notificationClient.MarkAsReadByType(ctx, req.UserId, groupInviteNotificationType, req.GroupId); err != nil {
		log.Printf("DeclineGroupInvite: best-effort mark-read failed: %v", err)
	}

	return &pb.DeclineGroupInviteResponse{}, nil
}

func (s *UserService) InviteGroupMembers(ctx context.Context, req *pb.InviteGroupMembersRequest) (*pb.InviteGroupMembersResponse, error) {
	group, err := s.groupRepo.GetGroupByID(ctx, req.GroupId)
	if err != nil {
		return nil, errors.New(codes.NotFound, errors.ReasonGroupNotFound, "Group not found", map[string]string{"group_id": req.GroupId})
	}

	if group.OwnerID != req.UserId {
		return nil, errors.New(codes.PermissionDenied, errors.ReasonUnauthorized, "Only group owner can invite members", map[string]string{"group_id": req.GroupId, "user_id": req.UserId})
	}

	if len(req.Emails) == 0 {
		return &pb.InviteGroupMembersResponse{}, nil
	}

	usersMap, err := s.userRepo.GetUsersByEmailsMap(ctx, req.Emails)
	if err != nil {
		log.Printf("InviteGroupMembers: failed to lookup users: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonGroupUpdateFailed, "Failed to resolve member emails", map[string]string{"group_id": req.GroupId})
	}

	memberIDs, err := s.groupRepo.GetMemberIDs(ctx, req.GroupId)
	if err != nil {
		log.Printf("InviteGroupMembers: failed to get members: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonGroupUpdateFailed, "Failed to retrieve members", map[string]string{"group_id": req.GroupId})
	}
	invitedIDs, err := s.groupRepo.GetInvitedUserIDs(ctx, req.GroupId)
	if err != nil {
		log.Printf("InviteGroupMembers: failed to get invitations: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonGroupUpdateFailed, "Failed to retrieve invitations", map[string]string{"group_id": req.GroupId})
	}

	existing := make(map[string]bool, len(memberIDs)+len(invitedIDs))
	for _, id := range memberIDs {
		existing[id] = true
	}
	for _, id := range invitedIDs {
		existing[id] = true
	}

	var invitedEmails []string
	err = s.txMgr.InTransaction(ctx, func(ctx context.Context) error {
		for _, email := range req.Emails {
			user, exists := usersMap[email]
			if !exists {
				continue
			}
			if user.ID == group.OwnerID || existing[user.ID] {
				continue
			}
			if err := s.groupRepo.AddInvitation(ctx, req.GroupId, user.ID, req.UserId); err != nil {
				return fmt.Errorf("add invitation: %w", err)
			}
			existing[user.ID] = true
			invitedEmails = append(invitedEmails, email)
		}
		return nil
	})
	if err != nil {
		log.Printf("InviteGroupMembers: tx failed: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonGroupUpdateFailed, "Failed to invite members", map[string]string{"group_id": req.GroupId})
	}

	for _, email := range invitedEmails {
		s.publishGroupInvite(ctx, group.ID, group.Name, req.UserId, email)
	}

	return &pb.InviteGroupMembersResponse{}, nil
}

func (s *UserService) KickGroupMembers(ctx context.Context, req *pb.KickGroupMembersRequest) (*pb.KickGroupMembersResponse, error) {
	group, err := s.groupRepo.GetGroupByID(ctx, req.GroupId)
	if err != nil {
		return nil, errors.New(codes.NotFound, errors.ReasonGroupNotFound, "Group not found", map[string]string{"group_id": req.GroupId})
	}

	if group.OwnerID != req.UserId {
		return nil, errors.New(codes.PermissionDenied, errors.ReasonUnauthorized, "Only group owner can kick members", map[string]string{"group_id": req.GroupId, "user_id": req.UserId})
	}

	if len(req.Emails) == 0 {
		return &pb.KickGroupMembersResponse{}, nil
	}

	usersMap, err := s.userRepo.GetUsersByEmailsMap(ctx, req.Emails)
	if err != nil {
		log.Printf("KickGroupMembers: failed to lookup users: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonGroupUpdateFailed, "Failed to resolve member emails", map[string]string{"group_id": req.GroupId})
	}

	for _, email := range req.Emails {
		user, exists := usersMap[email]
		if !exists {
			continue
		}
		if user.ID == group.OwnerID {
			return nil, errors.New(codes.PermissionDenied, errors.ReasonCannotKickOwner, "Cannot kick group owner", map[string]string{"group_id": req.GroupId, "user_id": user.ID})
		}
	}

	var kickedMembers []*repository.User
	err = s.txMgr.InTransaction(ctx, func(ctx context.Context) error {
		kickedMembers = kickedMembers[:0]
		for _, user := range usersMap {
			removed, err := s.groupRepo.RemoveMemberIfExists(ctx, req.GroupId, user.ID)
			if err != nil {
				return fmt.Errorf("remove member: %w", err)
			}
			if err := s.groupRepo.RemoveInvitation(ctx, req.GroupId, user.ID); err != nil {
				return fmt.Errorf("remove invitation: %w", err)
			}
			if removed {
				kickedMembers = append(kickedMembers, user)
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("KickGroupMembers: tx failed: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonGroupUpdateFailed, "Failed to kick members", map[string]string{"group_id": req.GroupId})
	}

	for _, user := range kickedMembers {
		s.publishGroupKicked(ctx, group.ID, group.Name, req.UserId, user)
		if err := s.notificationClient.MarkAsReadByType(ctx, user.ID, groupInviteNotificationType, group.ID); err != nil {
			log.Printf("KickGroupMembers: best-effort mark-read for stale invite failed for user %s: %v", user.ID, err)
		}
	}

	return &pb.KickGroupMembersResponse{}, nil
}

func (s *UserService) LeaveGroup(ctx context.Context, req *pb.LeaveGroupRequest) (*pb.LeaveGroupResponse, error) {
	group, err := s.groupRepo.GetGroupByID(ctx, req.GroupId)
	if err != nil {
		return nil, errors.New(codes.NotFound, errors.ReasonGroupNotFound, "Group not found", map[string]string{"group_id": req.GroupId})
	}

	if group.OwnerID == req.UserId {
		return nil, errors.New(codes.PermissionDenied, errors.ReasonOwnerCannotLeave, "Owner must delete the group instead of leaving", map[string]string{"group_id": req.GroupId})
	}

	isMember, err := s.groupRepo.IsMember(ctx, req.GroupId, req.UserId)
	if err != nil {
		log.Printf("LeaveGroup: failed to check membership: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonGroupUpdateFailed, "Failed to verify membership", map[string]string{"group_id": req.GroupId})
	}
	if !isMember {
		return nil, errors.New(codes.NotFound, errors.ReasonNotAMember, "Not a member of this group", map[string]string{"group_id": req.GroupId, "user_id": req.UserId})
	}

	if err := s.groupRepo.RemoveMember(ctx, req.GroupId, req.UserId); err != nil {
		log.Printf("LeaveGroup: failed to remove member: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonGroupUpdateFailed, "Failed to leave group", map[string]string{"group_id": req.GroupId})
	}

	return &pb.LeaveGroupResponse{}, nil
}

func (s *UserService) groupToProto(group *repository.Group) *pb.Group {
	return &pb.Group{
		Id:           group.ID,
		Name:         group.Name,
		Description:  group.Description,
		AvatarUrl:    group.AvatarURL,
		OwnerId:      group.OwnerID,
		CreatedAt:    group.CreatedAt.Unix(),
		MemberCount:  group.MemberCount,
		PendingCount: group.PendingCount,
	}
}

func (s *UserService) publishGroupKicked(ctx context.Context, groupID, groupName, kickerID string, kicked *repository.User) {
	if s.rabbitMQ == nil {
		log.Printf("publishGroupKicked: rabbitMQ is nil, skipping event for user %s", kicked.ID)
		return
	}

	kicker, err := s.userRepo.GetUserByID(ctx, kickerID)
	if err != nil {
		log.Printf("publishGroupKicked: failed to get kicker: %v", err)
		return
	}

	if kicked.IsRegistered {
		settings, settingsErr := s.settingsRepo.GetOrCreateSettings(ctx, kicked.ID)
		if settingsErr == nil && !settings.GroupKicked {
			log.Printf("publishGroupKicked: user %s opted out of group_kicked, skipping", kicked.ID)
			return
		}
	}

	kickedLanguage := string(lang.Default)
	if kicked.Language != "" {
		kickedLanguage = kicked.Language
	}

	event := map[string]string{
		"group_id":       groupID,
		"group_name":     groupName,
		"kicker_name":    displayName(kicker),
		"kicked_email":   kicked.Email,
		"kicked_user_id": kicked.ID,
		"language":       kickedLanguage,
	}
	eventData, _ := json.Marshal(event)

	if err := s.rabbitMQ.Publish(ctx, "user.group_kicks", eventData); err != nil {
		log.Printf("Failed to publish group_kicked event: %v", err)
	}
}

func (s *UserService) publishGroupInvite(ctx context.Context, groupID, groupName, inviterID, inviteeEmail string) {
	if s.rabbitMQ == nil {
		log.Printf("publishGroupInvite: rabbitMQ is nil, skipping event for invitee %s", inviteeEmail)
		return
	}

	inviter, err := s.userRepo.GetUserByID(ctx, inviterID)
	if err != nil {
		log.Printf("Failed to get inviter: %v", err)
		return
	}

	inviteeUserID := ""
	inviteeLanguage := string(lang.Default)
	if invitee, err := s.userRepo.GetUserByEmail(ctx, inviteeEmail); err == nil && invitee != nil {
		if invitee.IsRegistered {
			settings, settingsErr := s.settingsRepo.GetOrCreateSettings(ctx, invitee.ID)
			if settingsErr == nil && !settings.GroupInvites {
				log.Printf("publishGroupInvite: user %s opted out of group_invites, skipping", invitee.ID)
				return
			}
		}
		inviteeUserID = invitee.ID
		if invitee.Language != "" {
			inviteeLanguage = invitee.Language
		}
	}

	event := map[string]string{
		"group_id":        groupID,
		"group_name":      groupName,
		"inviter_name":    displayName(inviter),
		"invitee_email":   inviteeEmail,
		"invitee_user_id": inviteeUserID,
		"language":        inviteeLanguage,
	}
	eventData, _ := json.Marshal(event)

	if err := s.rabbitMQ.Publish(ctx, "user.group_invites", eventData); err != nil {
		log.Printf("Failed to publish group_invite event: %v", err)
	}
}

func displayName(u *repository.User) string {
	name := u.FirstName + " " + u.LastName
	if name == " " {
		return u.Email
	}
	return name
}

func (s *UserService) validateGroupAvatarURL(url string) error {
	if url == "" {
		return nil
	}
	expectedPrefix := s.s3Client.GetPublicURL(constants.GroupAvatarBucketName, "")
	if !strings.HasPrefix(url, expectedPrefix) {
		return errors.New(codes.InvalidArgument, errors.ReasonInvalidAvatarURL, "avatar_url must originate from the group-avatars bucket upload endpoint", map[string]string{"avatar_url": url})
	}
	return nil
}

func (s *UserService) deleteGroupAvatarFile(ctx context.Context, avatarURL string) error {
	bucketPrefix := "/" + constants.GroupAvatarBucketName + "/"
	_, after, ok := strings.Cut(avatarURL, bucketPrefix)
	if !ok {
		log.Printf("deleteGroupAvatarFile: avatar URL %q does not contain bucket prefix, skipping S3 delete", avatarURL)
		return nil
	}
	return s.s3Client.DeleteFile(ctx, constants.GroupAvatarBucketName, after)
}

func contentTypeForFilename(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}
