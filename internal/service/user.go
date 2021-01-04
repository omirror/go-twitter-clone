package service

import (
    "context"
    "errors"
    "fmt"
    "regexp"
    "strings"
)

var (
    rxEmail    = regexp.MustCompile("^[^\\s@]+@[^\\s@]+\\.[^\\s@]+$")
    rxUsername = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9_-]{0,17}$")

    // ErrUserNotFound is used to indicate that user wasn't found at database.
    ErrUserNotFound = errors.New("user not found")
    //ErrInvalidEmail is used to indicate that email pattern is incorrect.
    ErrInvalidEmail = errors.New("Invalid Email Pattern")
    // ErrEmailNotUnique is used to indicate that user can't proceed creating this eamil as it exists.
    ErrEmailNotUnique = errors.New("Email isn't unique, use another email")
    // ErrUsernameNotUnique is used to indicate that user can't proceed creating this eamil as it exists.
    ErrUsernameNotUnique = errors.New("Username isn't unique, use another username")
    //ErrInvalidUsername is used to indicate that username pattern is incorrect.
    ErrInvalidUsername = errors.New("Invalid Username Pattern")
)

// User Model.
type User struct {
    ID       int64  `json:"id"`
    Username string `json:"username"`
}

// CreateUser is used to create a user.
func (s *Service) CreateUser(ctx context.Context, email, username string) error {

    email = strings.TrimSpace(email)
    if !rxEmail.MatchString(email) {
        return ErrInvalidEmail
    }
    username = strings.TrimSpace(username)
    if !rxUsername.MatchString(username) {
        return ErrInvalidUsername
    }
    query := "INSERT INTO users (email, username) VALUES($1, $2)"
    _, err := s.db.ExecContext(ctx, query, email, username)
    unique := isUniqueViolation(err)
    if unique && strings.Contains(err.Error(), "email") {
        return ErrEmailNotUnique
    }
    if unique && strings.Contains(err.Error(), "username") {
        return ErrUsernameNotUnique
    }
    if err != nil {
        return fmt.Errorf("couldn't insert user: %v", err)
    }
    return nil
}
