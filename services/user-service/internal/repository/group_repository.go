package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"user-service/pkg/database"
)

type Group struct {
	ID           string
	Name         string
	Description  string
	AvatarURL    string
	OwnerID      string
	CreatedAt    time.Time
	MemberCount  int32
	PendingCount int32
}

type GroupMember struct {
	GroupID  string
	UserID   string
	JoinedAt time.Time
}

type GroupInvitation struct {
	GroupID   string
	UserID    string
	InviterID string
	InvitedAt time.Time
}

type GroupRepository struct {
	db database.DBTX
}

func NewGroupRepository(db database.DBTX) *GroupRepository {
	return &GroupRepository{db: db}
}

func (r *GroupRepository) q(ctx context.Context) database.DBTX {
	return database.Querier(ctx, r.db)
}

func (r *GroupRepository) CreateGroup(ctx context.Context, name, description, avatarURL, ownerID string) (*Group, error) {
	group := &Group{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		AvatarURL:   avatarURL,
		OwnerID:     ownerID,
		CreatedAt:   time.Now(),
	}

	query := `
		INSERT INTO groups (id, name, description, avatar_url, owner_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, description, avatar_url, owner_id, created_at
	`

	err := r.q(ctx).QueryRowContext(ctx, query,
		group.ID,
		group.Name,
		group.Description,
		group.AvatarURL,
		group.OwnerID,
		group.CreatedAt,
	).Scan(
		&group.ID,
		&group.Name,
		&group.Description,
		&group.AvatarURL,
		&group.OwnerID,
		&group.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create group: %w", err)
	}

	return group, nil
}

func (r *GroupRepository) GetGroupByID(ctx context.Context, groupID string) (*Group, error) {
	query := `
		SELECT
			g.id,
			g.name,
			g.description,
			g.avatar_url,
			g.owner_id,
			g.created_at,
			(SELECT COUNT(*) FROM group_members gm WHERE gm.group_id = g.id) AS member_count,
			(SELECT COUNT(*) FROM group_invitations gi WHERE gi.group_id = g.id) AS pending_count
		FROM groups g
		WHERE g.id = $1
	`

	group := &Group{}
	err := r.q(ctx).QueryRowContext(ctx, query, groupID).Scan(
		&group.ID,
		&group.Name,
		&group.Description,
		&group.AvatarURL,
		&group.OwnerID,
		&group.CreatedAt,
		&group.MemberCount,
		&group.PendingCount,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("group not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get group: %w", err)
	}

	return group, nil
}

func (r *GroupRepository) UpdateGroup(ctx context.Context, group *Group) error {
	query := `
		UPDATE groups
		SET name = $2, description = $3, avatar_url = $4
		WHERE id = $1
	`

	result, err := r.q(ctx).ExecContext(ctx, query, group.ID, group.Name, group.Description, group.AvatarURL)
	if err != nil {
		return fmt.Errorf("failed to update group: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("group not found")
	}

	return nil
}

func (r *GroupRepository) DeleteGroup(ctx context.Context, groupID string) error {
	query := `DELETE FROM groups WHERE id = $1`

	result, err := r.q(ctx).ExecContext(ctx, query, groupID)
	if err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("group not found")
	}

	return nil
}

func (r *GroupRepository) AddMember(ctx context.Context, groupID, userID string) error {
	query := `
		INSERT INTO group_members (group_id, user_id, joined_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (group_id, user_id) DO NOTHING
	`

	_, err := r.q(ctx).ExecContext(ctx, query, groupID, userID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to add member: %w", err)
	}

	return nil
}

func (r *GroupRepository) AddMembers(ctx context.Context, groupID string, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}

	valueStrings := make([]string, 0, len(userIDs))
	valueArgs := make([]any, 0, len(userIDs)*3)
	now := time.Now()

	for i, userID := range userIDs {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d)", i*3+1, i*3+2, i*3+3))
		valueArgs = append(valueArgs, groupID, userID, now)
	}

	query := fmt.Sprintf(`
		INSERT INTO group_members (group_id, user_id, joined_at)
		VALUES %s
		ON CONFLICT (group_id, user_id) DO NOTHING
	`, strings.Join(valueStrings, ", "))

	_, err := r.q(ctx).ExecContext(ctx, query, valueArgs...)
	if err != nil {
		return fmt.Errorf("failed to add members: %w", err)
	}

	return nil
}

func (r *GroupRepository) RemoveMember(ctx context.Context, groupID, userID string) error {
	query := `DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`

	result, err := r.q(ctx).ExecContext(ctx, query, groupID, userID)
	if err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("member not found in group")
	}

	return nil
}

func (r *GroupRepository) RemoveMemberIfExists(ctx context.Context, groupID, userID string) (bool, error) {
	query := `DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`

	result, err := r.q(ctx).ExecContext(ctx, query, groupID, userID)
	if err != nil {
		return false, fmt.Errorf("failed to remove member: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected > 0, nil
}

func (r *GroupRepository) GetMemberIDs(ctx context.Context, groupID string) ([]string, error) {
	query := `
		SELECT user_id
		FROM group_members
		WHERE group_id = $1
	`

	rows, err := r.q(ctx).QueryContext(ctx, query, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to query members: %w", err)
	}
	defer rows.Close()

	var memberIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("failed to scan user_id: %w", err)
		}
		memberIDs = append(memberIDs, userID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return memberIDs, nil
}

func (r *GroupRepository) GetMemberCount(ctx context.Context, groupID string) (int32, error) {
	query := `
		SELECT COUNT(*) AS count
		FROM group_members
		WHERE group_id = $1
	`

	var count int32
	err := r.q(ctx).QueryRowContext(ctx, query, groupID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get member count: %w", err)
	}

	return count, nil
}

func (r *GroupRepository) GetUserGroups(ctx context.Context, userID string) ([]*Group, error) {
	query := `
		SELECT
			g.id,
			g.name,
			g.description,
			g.avatar_url,
			g.owner_id,
			g.created_at,
			(SELECT COUNT(*) FROM group_members gm2 WHERE gm2.group_id = g.id) AS member_count,
			(SELECT COUNT(*) FROM group_invitations gi WHERE gi.group_id = g.id) AS pending_count
		FROM groups g
		INNER JOIN group_members gm ON g.id = gm.group_id
		WHERE gm.user_id = $1
	`

	return r.scanGroups(ctx, query, userID)
}

func (r *GroupRepository) GetCreatedGroups(ctx context.Context, ownerID string) ([]*Group, error) {
	query := `
		SELECT
			g.id,
			g.name,
			g.description,
			g.avatar_url,
			g.owner_id,
			g.created_at,
			(SELECT COUNT(*) FROM group_members gm WHERE gm.group_id = g.id) AS member_count,
			(SELECT COUNT(*) FROM group_invitations gi WHERE gi.group_id = g.id) AS pending_count
		FROM groups g
		WHERE g.owner_id = $1
	`

	return r.scanGroups(ctx, query, ownerID)
}

func (r *GroupRepository) GetInvitedGroups(ctx context.Context, userID string) ([]*Group, error) {
	query := `
		SELECT
			g.id,
			g.name,
			g.description,
			g.avatar_url,
			g.owner_id,
			g.created_at,
			(SELECT COUNT(*) FROM group_members gm WHERE gm.group_id = g.id) AS member_count,
			(SELECT COUNT(*) FROM group_invitations gi2 WHERE gi2.group_id = g.id) AS pending_count
		FROM groups g
		INNER JOIN group_invitations gi ON g.id = gi.group_id
		WHERE gi.user_id = $1
	`

	return r.scanGroups(ctx, query, userID)
}

func (r *GroupRepository) scanGroups(ctx context.Context, query string, args ...any) ([]*Group, error) {
	rows, err := r.q(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query groups: %w", err)
	}
	defer rows.Close()

	var groups []*Group
	for rows.Next() {
		var group Group
		if err := rows.Scan(
			&group.ID,
			&group.Name,
			&group.Description,
			&group.AvatarURL,
			&group.OwnerID,
			&group.CreatedAt,
			&group.MemberCount,
			&group.PendingCount,
		); err != nil {
			return nil, fmt.Errorf("failed to scan group: %w", err)
		}
		groups = append(groups, &group)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return groups, nil
}

func (r *GroupRepository) IsMember(ctx context.Context, groupID, userID string) (bool, error) {
	query := `
		SELECT COUNT(*) AS count
		FROM group_members
		WHERE group_id = $1 AND user_id = $2
	`

	var count int32
	err := r.q(ctx).QueryRowContext(ctx, query, groupID, userID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check membership: %w", err)
	}

	return count > 0, nil
}

func (r *GroupRepository) DeleteOwnedGroups(ctx context.Context, ownerID string) error {
	query := `DELETE FROM groups WHERE owner_id = $1`
	if _, err := r.q(ctx).ExecContext(ctx, query, ownerID); err != nil {
		return fmt.Errorf("failed to delete owned groups: %w", err)
	}
	return nil
}

func (r *GroupRepository) DeleteUserMemberships(ctx context.Context, userID string) error {
	query := `DELETE FROM group_members WHERE user_id = $1`
	if _, err := r.q(ctx).ExecContext(ctx, query, userID); err != nil {
		return fmt.Errorf("failed to delete user memberships: %w", err)
	}
	return nil
}

func (r *GroupRepository) GetGroupUsers(ctx context.Context, groupID string) ([]*User, error) {
	query := `
		SELECT
			u.id,
			u.email,
			u.first_name,
			u.last_name,
			u.avatar_url,
			u.is_registered,
			u.created_at
		FROM users u
		INNER JOIN group_members gm ON u.id = gm.user_id
		WHERE gm.group_id = $1
		ORDER BY gm.joined_at
	`

	return r.scanUsers(ctx, query, groupID)
}

func (r *GroupRepository) GetGroupInvitedUsers(ctx context.Context, groupID string) ([]*User, error) {
	query := `
		SELECT
			u.id,
			u.email,
			u.first_name,
			u.last_name,
			u.avatar_url,
			u.is_registered,
			u.created_at
		FROM users u
		INNER JOIN group_invitations gi ON u.id = gi.user_id
		WHERE gi.group_id = $1
		ORDER BY gi.invited_at
	`

	return r.scanUsers(ctx, query, groupID)
}

func (r *GroupRepository) scanUsers(ctx context.Context, query string, args ...any) ([]*User, error) {
	rows, err := r.q(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.AvatarURL, &user.IsRegistered, &user.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, &user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return users, nil
}

func (r *GroupRepository) AddInvitation(ctx context.Context, groupID, userID, inviterID string) error {
	query := `
		INSERT INTO group_invitations (group_id, user_id, inviter_id, invited_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (group_id, user_id) DO NOTHING
	`

	if _, err := r.q(ctx).ExecContext(ctx, query, groupID, userID, inviterID, time.Now()); err != nil {
		return fmt.Errorf("failed to add invitation: %w", err)
	}

	return nil
}

func (r *GroupRepository) RemoveInvitation(ctx context.Context, groupID, userID string) error {
	query := `DELETE FROM group_invitations WHERE group_id = $1 AND user_id = $2`

	if _, err := r.q(ctx).ExecContext(ctx, query, groupID, userID); err != nil {
		return fmt.Errorf("failed to remove invitation: %w", err)
	}

	return nil
}

func (r *GroupRepository) IsInvited(ctx context.Context, groupID, userID string) (bool, error) {
	query := `
		SELECT COUNT(*) AS count
		FROM group_invitations
		WHERE group_id = $1 AND user_id = $2
	`

	var count int32
	err := r.q(ctx).QueryRowContext(ctx, query, groupID, userID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check invitation: %w", err)
	}

	return count > 0, nil
}

func (r *GroupRepository) AcceptInvitation(ctx context.Context, groupID, userID string) error {
	deleteQ := `DELETE FROM group_invitations WHERE group_id = $1 AND user_id = $2`
	result, err := r.q(ctx).ExecContext(ctx, deleteQ, groupID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete invitation: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("invitation not found")
	}

	insertQ := `
		INSERT INTO group_members (group_id, user_id, joined_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (group_id, user_id) DO NOTHING
	`
	if _, err := r.q(ctx).ExecContext(ctx, insertQ, groupID, userID, time.Now()); err != nil {
		return fmt.Errorf("failed to add member after accept: %w", err)
	}

	return nil
}

func (r *GroupRepository) GetInvitedUserIDs(ctx context.Context, groupID string) ([]string, error) {
	query := `SELECT user_id FROM group_invitations WHERE group_id = $1`

	rows, err := r.q(ctx).QueryContext(ctx, query, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to query invited user ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan user_id: %w", err)
		}
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return ids, nil
}

func (r *GroupRepository) DeleteUserInvitations(ctx context.Context, userID string) error {
	query := `DELETE FROM group_invitations WHERE user_id = $1`
	if _, err := r.q(ctx).ExecContext(ctx, query, userID); err != nil {
		return fmt.Errorf("failed to delete user invitations: %w", err)
	}
	return nil
}