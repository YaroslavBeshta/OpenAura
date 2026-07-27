package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openaura/openaura/internal/httpx"
	"github.com/openaura/openaura/internal/store"
)

var (
	ErrInvalidEmail = errors.New("invalid email")
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, appID uuid.UUID, in CreateInput) (User, error) {
	email, err := normalizeEmail(in.Email)
	if err != nil {
		return User{}, err
	}
	metadata, err := httpx.NormalizeMetadata(in.Metadata)
	if err != nil {
		return User{}, fmt.Errorf("%w: metadata must be a JSON object", store.ErrInvalidInput)
	}
	if appID == uuid.Nil {
		return User{}, fmt.Errorf("%w: app_id is required", store.ErrInvalidInput)
	}

	now := time.Now().UTC()
	id, err := uuid.NewV7()
	if err != nil {
		return User{}, fmt.Errorf("generate uuid: %w", err)
	}

	const q = `
		INSERT INTO users (id, app_id, email, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, app_id, email, metadata, created_at, updated_at, deleted_at`

	var u User
	err = r.pool.QueryRow(ctx, q, id, appID, email, metadata, now, now).Scan(
		&u.ID, &u.AppID, &u.Email, &u.Metadata, &u.CreatedAt, &u.UpdatedAt, &u.DeletedAt,
	)
	if err != nil {
		return User{}, store.MapWriteError(err)
	}
	return u, nil
}

func (r *Repository) GetByID(ctx context.Context, appID, id uuid.UUID) (User, error) {
	const q = `
		SELECT id, app_id, email, metadata, created_at, updated_at, deleted_at
		FROM users
		WHERE id = $1 AND app_id = $2 AND deleted_at IS NULL`

	var u User
	err := r.pool.QueryRow(ctx, q, id, appID).Scan(
		&u.ID, &u.AppID, &u.Email, &u.Metadata, &u.CreatedAt, &u.UpdatedAt, &u.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, store.ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

func (r *Repository) List(ctx context.Context, f ListFilter) ([]User, error) {
	limit, offset := clampPagination(f.Limit, f.Offset)
	const q = `
		SELECT id, app_id, email, metadata, created_at, updated_at, deleted_at
		FROM users
		WHERE app_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, q, f.AppID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.AppID, &u.Email, &u.Metadata, &u.CreatedAt, &u.UpdatedAt, &u.DeletedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *Repository) Update(ctx context.Context, appID, id uuid.UUID, in UpdateInput) (User, error) {
	if in.Email == nil && in.Metadata == nil {
		return User{}, fmt.Errorf("%w: email or metadata is required", store.ErrInvalidInput)
	}

	current, err := r.GetByID(ctx, appID, id)
	if err != nil {
		return User{}, err
	}

	email := current.Email
	if in.Email != nil {
		email, err = normalizeEmail(*in.Email)
		if err != nil {
			return User{}, err
		}
	}

	metadata := current.Metadata
	if in.Metadata != nil {
		metadata, err = httpx.NormalizeMetadata(*in.Metadata)
		if err != nil {
			return User{}, fmt.Errorf("%w: metadata must be a JSON object", store.ErrInvalidInput)
		}
	}

	const q = `
		UPDATE users
		SET email = $1, metadata = $2, updated_at = $3
		WHERE id = $4 AND app_id = $5 AND deleted_at IS NULL
		RETURNING id, app_id, email, metadata, created_at, updated_at, deleted_at`

	var u User
	err = r.pool.QueryRow(ctx, q, email, metadata, time.Now().UTC(), id, appID).Scan(
		&u.ID, &u.AppID, &u.Email, &u.Metadata, &u.CreatedAt, &u.UpdatedAt, &u.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, store.ErrNotFound
	}
	if err != nil {
		return User{}, store.MapWriteError(err)
	}
	return u, nil
}

func (r *Repository) SoftDelete(ctx context.Context, appID, id uuid.UUID) error {
	const q = `
		UPDATE users
		SET deleted_at = $1, updated_at = $1
		WHERE id = $2 AND app_id = $3 AND deleted_at IS NULL`

	tag, err := r.pool.Exec(ctx, q, time.Now().UTC(), id, appID)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

func normalizeEmail(email string) (string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || !strings.Contains(email, "@") {
		return "", ErrInvalidEmail
	}
	return email, nil
}

func clampPagination(limit, offset int) (int, int) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
