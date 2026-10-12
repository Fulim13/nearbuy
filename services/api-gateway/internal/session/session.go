package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/Fulim13/nearbuy/pkg/grpx"
	"github.com/redis/go-redis/v9"
)

const (
	codeKeyPrefix  = "login:code:"
	tokenKeyPrefix = "login:token:"

	CodeTTL  = 2 * time.Minute
	TokenTTL = 30 * time.Minute
)

var ErrCodeMismatch = errors.New("code is wrong or expired")

type User struct {
	ID       int64  `json:"id"`
	NickName string `json:"nickname"`
	Icon     string `json:"icon"`
	// "user" or "admin". Sessions created before roles existed have "", which
	// IsAdmin treats as a plain user.
	Role string `json:"role"`
}

func (u User) IsAdmin() bool { return u.Role == grpx.RoleAdmin }

type Store struct {
	rdb *redis.Client
}

func NewStore(rdb *redis.Client) *Store {
	return &Store{rdb: rdb}
}

func NewCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func (s *Store) SaveCode(ctx context.Context, phone, code string) error {
	return s.rdb.Set(ctx, codeKeyPrefix+phone, code, CodeTTL).Err()
}

func (s *Store) VerifyCode(ctx context.Context, phone, code string) error {
	key := codeKeyPrefix + phone
	stored, err := s.rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return ErrCodeMismatch
	}
	if err != nil {
		return err
	}
	if stored != code {
		return ErrCodeMismatch
	}
	return s.rdb.Del(ctx, key).Err()
}

func (s *Store) Create(ctx context.Context, u User) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	payload, err := json.Marshal(u)
	if err != nil {
		return "", err
	}
	if err := s.rdb.Set(ctx, tokenKeyPrefix+token, payload, TokenTTL).Err(); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Store) Get(ctx context.Context, token string) (User, error) {
	key := tokenKeyPrefix + token
	payload, err := s.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return User{}, err
	}
	var u User
	if err := json.Unmarshal(payload, &u); err != nil {
		return User{}, err
	}
	// Sliding window: an active user is never logged out mid-session.
	s.rdb.Expire(ctx, key, TokenTTL)
	return u, nil
}

func (s *Store) Delete(ctx context.Context, token string) error {
	return s.rdb.Del(ctx, tokenKeyPrefix+token).Err()
}

func NewNickname() (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	out := make([]byte, 8)
	for i := range out {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		out[i] = alphabet[n.Int64()]
	}
	return "user_" + string(out), nil
}
