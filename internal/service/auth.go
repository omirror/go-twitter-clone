package service

import (
    "context"
    "database/sql"
    "fmt"
    "strconv"
    "strings"
    "time"
)

const (
    // TokenTTL until token expires.
    TokenTTL = time.Hour * 24 * 14
)

// LoginOutput is the login response.
type LoginOutput struct {
    Token     string    `json:"token"`
    ExpiresAt time.Time `json:"expires_at"`
    User      User      `json:"user"`
}

// Login user
func (s *Service) Login(ctx context.Context, email string) (LoginOutput, error) {
    var response LoginOutput
    email = strings.TrimSpace(email)
    if !rxEmail.MatchString(email) {
        return response, ErrInvalidEmail
    }
    query := "SELECT id, username FROM users where email = $1"
    err := s.db.QueryRowContext(ctx, query, email).Scan(&response.User.ID, &response.User.Username)
    if err == sql.ErrNoRows {
        return response, ErrUserNotFound
    }
    if err != nil {
        return response, fmt.Errorf("could not query select user: %v", err)
    }
    response.Token, err = s.codec.EncodeToString(strconv.FormatInt(response.User.ID, 10))
    if err != nil {
        return response, fmt.Errorf("couldn't create token: %v", err)
    }
    response.ExpiresAt = time.Now().Add(TokenTTL)
    return response, nil
}
