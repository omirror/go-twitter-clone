package service

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "strconv"
    "strings"
    "time"
)

const (
    // TokenTTL until token expires.
    TokenTTL = time.Hour * 24 * 14
    // KeyAuthUserID is used to identify the auth_user_key
    KeyAuthUserID key = "auth_user_id"
)

var (
    //ErrUnauthenticated is used when the user isn't authenticated, and trying to access something requires authentication.
    ErrUnauthenticated = errors.New("unauthenticated")
)

type key string

// LoginOutput is the login response.
type LoginOutput struct {
    Token     string    `json:"token"`
    ExpiresAt time.Time `json:"expires_at"`
    User      User      `json:"user"`
}

//AuthUserID from token
func (s *Service) AuthUserID(token string) (int64, error) {
    str, err := s.codec.DecodeToString(token)
    if err != nil {
        return 0, fmt.Errorf("could not decode token: %v", err)
    }
    i, err := strconv.ParseInt(str, 10, 64)
    if err != nil {
        return 0, fmt.Errorf("Couldn't parse auth user id from token: %v", err)
    }
    return i, nil
}

// Login user
func (s *Service) Login(ctx context.Context, email string) (LoginOutput, error) {
    var response LoginOutput
    email = strings.TrimSpace(email)
    if !rxEmail.MatchString(email) {
        return response, ErrInvalidEmail
    }
    var avatar sql.NullString
    query := "SELECT id, username, avatar FROM users where email = $1"
    err := s.db.QueryRowContext(ctx, query, email).Scan(&response.User.ID, &response.User.Username, &avatar)
    if err == sql.ErrNoRows {
        return response, ErrUserNotFound
    }
    if err != nil {
        return response, fmt.Errorf("could not query select user: %v", err)
    }
    if avatar.Valid {
        avatarURL := s.origin + "/public/avatars/users/" + avatar.String
        response.User.AvatarURL = &avatarURL
    }
    response.Token, err = s.codec.EncodeToString(strconv.FormatInt(response.User.ID, 10))
    if err != nil {
        return response, fmt.Errorf("couldn't create token: %v", err)
    }
    response.ExpiresAt = time.Now().Add(TokenTTL)
    return response, nil
}

//AuthUser from context
func (s *Service) AuthUser(ctx context.Context) (User, error) {
    var u User
    uid, ok := ctx.Value(KeyAuthUserID).(int64)
    if !ok {
        return u, ErrUnauthenticated
    }
    var avatar sql.NullString
    query := "SELECT username, avatar from users WHERE id = $1"
    err := s.db.QueryRowContext(ctx, query, uid).Scan(&u.Username, &avatar)
    if err == sql.ErrNoRows {
        return u, ErrUserNotFound
    }
    if err != nil {
        return u, fmt.Errorf("couldn't query select auth user: %v", err)
    }
    if avatar.Valid {
        avatarURL := s.origin + "/public/avatars/users/" + avatar.String
        u.AvatarURL = &avatarURL
    }
    u.ID = uid
    return u, nil
}
