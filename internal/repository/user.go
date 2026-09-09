package repository

import "github.com/jackc/pgx/v5/pgxpool"

type UserRepository interface {
	GetOneById()
	GetOneByLogin()
}

type userRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) UserRepository {
	return &userRepository{db: db}
}

func (u *userRepository) GetOneById() {

}

func (u *userRepository) GetOneByLogin() {

}
