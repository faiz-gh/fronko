package repository

import (
	"context"

	"github.com/faiz-gh/credensync/backend/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	// TODO: implement SQL query
	return nil, nil
}

func (r *Repository) CreateUser(ctx context.Context, user *models.User) error {
	// TODO: implement SQL query
	return nil
}

func (r *Repository) GetProfile(ctx context.Context, id string) (*models.Profile, error) {
	// TODO: implement SQL query
	return nil, nil
}
