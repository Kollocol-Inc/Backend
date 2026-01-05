package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Group struct {
	ID          string
	Name        string
	OwnerID     string
	CreatedAt   time.Time
	MemberCount int32
}

type GroupMember struct {
	GroupID  string
	UserID   string
	JoinedAt time.Time
}

type GroupRepository struct {
	db *sql.DB
}

func NewGroupRepository(db *sql.DB) *GroupRepository {
	return &GroupRepository{db: db}
}

func (r *GroupRepository) CreateGroup(ctx context.Context, name, ownerID string) (*Group, error) {
	group := &Group{
		ID:        uuid.New().String(),
		Name:      name,
		OwnerID:   ownerID,
		CreatedAt: time.Now(),
	}

	query := `
		INSERT INTO groups (id, name, owner_id, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, owner_id, created_at
	`

	err := r.db.QueryRowContext(ctx, query,
		group.ID,
		group.Name,
		group.OwnerID,
		group.CreatedAt,
	).Scan(
		&group.ID,
		&group.Name,
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
			g.owner_id,
			g.created_at,
			(SELECT COUNT(*) FROM group_members gm WHERE gm.group_id = g.id) AS member_count
		FROM groups g
		WHERE g.id = $1
	`

	group := &Group{}
	err := r.db.QueryRowContext(ctx, query, groupID).Scan(
		&group.ID,
		&group.Name,
		&group.OwnerID,
		&group.CreatedAt,
		&group.MemberCount,
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
		SET name = $2
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, group.ID, group.Name)
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

	result, err := r.db.ExecContext(ctx, query, groupID)
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

	_, err := r.db.ExecContext(ctx, query, groupID, userID, time.Now())
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

	_, err := r.db.ExecContext(ctx, query, valueArgs...)
	if err != nil {
		return fmt.Errorf("failed to add members: %w", err)
	}

	return nil
}

func (r *GroupRepository) RemoveMember(ctx context.Context, groupID, userID string) error {
	query := `DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`

	result, err := r.db.ExecContext(ctx, query, groupID, userID)
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

func (r *GroupRepository) GetMemberIDs(ctx context.Context, groupID string) ([]string, error) {
	query := `
		SELECT user_id
		FROM group_members
		WHERE group_id = $1
	`

	rows, err := r.db.QueryContext(ctx, query, groupID)
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
	err := r.db.QueryRowContext(ctx, query, groupID).Scan(&count)
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
			g.owner_id,
			g.created_at,
			(SELECT COUNT(*) FROM group_members gm2 WHERE gm2.group_id = g.id) AS member_count
		FROM groups g
		INNER JOIN group_members gm ON g.id = gm.group_id
		WHERE gm.user_id = $1
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query groups: %w", err)
	}
	defer rows.Close()

	var groups []*Group
	for rows.Next() {
		var group Group
		if err := rows.Scan(&group.ID, &group.Name, &group.OwnerID, &group.CreatedAt, &group.MemberCount); err != nil {
			return nil, fmt.Errorf("failed to scan group: %w", err)
		}
		groups = append(groups, &group)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return groups, nil
}

func (r *GroupRepository) GetCreatedGroups(ctx context.Context, ownerID string) ([]*Group, error) {
	query := `
		SELECT
			g.id,
			g.name,
			g.owner_id,
			g.created_at,
			(SELECT COUNT(*) FROM group_members gm WHERE gm.group_id = g.id) AS member_count
		FROM groups g
		WHERE g.owner_id = $1
	`

	rows, err := r.db.QueryContext(ctx, query, ownerID)
	if err != nil {
		return nil, fmt.Errorf("failed to query groups: %w", err)
	}
	defer rows.Close()

	var groups []*Group
	for rows.Next() {
		var group Group
		if err := rows.Scan(&group.ID, &group.Name, &group.OwnerID, &group.CreatedAt, &group.MemberCount); err != nil {
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
	err := r.db.QueryRowContext(ctx, query, groupID, userID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check membership: %w", err)
	}

	return count > 0, nil
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
	`

	rows, err := r.db.QueryContext(ctx, query, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to query group users: %w", err)
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