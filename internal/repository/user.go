package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mrbeaver1/dock-service/internal/models"
)

type UserRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (models.User, error)
	GetByLogin(ctx context.Context, login string) (models.User, error)
	Create(ctx context.Context, user models.User) (models.User, error)
}

type userRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) UserRepository {
	return &userRepository{db: db}
}

func (u *userRepository) GetByID(ctx context.Context, id uuid.UUID) (models.User, error) {
	var user models.User
	err := u.db.QueryRow(ctx, `SELECT id, login, password_hash, created_at FROM users WHERE id = $1`, id).
		Scan(&user.ID, &user.Login, &user.PasswordHash, &user.CreatedAt)
	return user, queryError("get user", err, models.ErrUserNotFound)
}

func (u *userRepository) GetByLogin(ctx context.Context, login string) (models.User, error) {
	var user models.User
	err := u.db.QueryRow(ctx, `SELECT id, login, password_hash, created_at FROM users WHERE login = $1`, login).
		Scan(&user.ID, &user.Login, &user.PasswordHash, &user.CreatedAt)
	return user, queryError("get user by login", err, models.ErrUserNotFound)
}

func (u *userRepository) Create(ctx context.Context, user models.User) (models.User, error) {
	if user.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return models.User{}, fmt.Errorf("generate user ID: %w", err)
		}
		user.ID = id
	}
	err := u.db.QueryRow(ctx, `INSERT INTO users (id, login, password_hash) VALUES ($1, $2, $3) RETURNING created_at`,
		user.ID, user.Login, user.PasswordHash).Scan(&user.CreatedAt)
	return user, queryError("create user", err, models.ErrUserNotFound)
}
