package service

import (
	"context"
	"errors"

	"github.com/Fulim13/nearbuy/services/user-service/internal/repository"
	"github.com/jackc/pgx/v5"
)

var ErrUserNotFound = errors.New("user not found")

type DB interface {
	repository.DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
}

type UserService struct {
	db   DB
	repo *repository.Queries
}

func NewUserService(db DB) *UserService {
	return &UserService{db: db, repo: repository.New(db)}
}

func (s *UserService) withTx(ctx context.Context, fn func(q *repository.Queries) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := fn(s.repo.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *UserService) GetUser(ctx context.Context, id int64) (repository.AppUser, error) {
	u, err := s.repo.GetUser(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return repository.AppUser{}, ErrUserNotFound
	}
	return u, err
}

// The insert's DO UPDATE locks the row.
// If every login used it, two logins for the same existing user would have to wait for each other.
// The read takes no lock, so normal logins stay fast. Only brand-new phones hit the insert.
func (s *UserService) GetOrCreateUserByPhone(ctx context.Context, phone, defaultNick string) (repository.AppUser, bool, error) {
	u, err := s.repo.GetUserByPhone(ctx, phone)
	if err == nil {
		return u, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return repository.AppUser{}, false, err
	}

	// Two first-time logins for the same phone arrive together, both see "not found", both try to insert.
	// The ON CONFLICT makes sure both get the same user back instead of one failing.
	u, err = s.repo.CreateUserByPhone(ctx, repository.CreateUserByPhoneParams{
		Phone:    phone,
		NickName: defaultNick,
	})
	return u, err == nil, err
}
