package service

import (
    "context"
    "database/sql"
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
    // ErrForbiddenFollow is used to indicate that user can't follow himself
    ErrForbiddenFollow = errors.New("You can not follow yourself")
)

// User Model.
type User struct {
    ID       int64  `json:"id"`
    Username string `json:"username"`
}

//ToggleFollowResponse is used to show the response of toggling a follow of a user.
type ToggleFollowResponse struct {
    Following      bool `json:"following"`
    FollowersCount int  `json:"followers_count"`
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

//ToggleFollow is used to toggle follow of a certain user throughout the authenticated user.
func (s *Service) ToggleFollow(ctx context.Context, username string) (ToggleFollowResponse, error) {
    var response ToggleFollowResponse
    followerID, ok := ctx.Value(KeyAuthUserID).(int64)
    if !ok {
        return response, ErrUnauthenticated
    }
    username = strings.TrimSpace(username)
    if !rxUsername.MatchString(username) {
        return response, ErrInvalidUsername
    }
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return response, fmt.Errorf("Couldn't begin tx: %v", err)
    }
    defer tx.Rollback()
    var followeeID int64
    query := "SELECT id FROM users where username = $1"
    err = tx.QueryRowContext(ctx, query, username).Scan(&followeeID)
    if err == sql.ErrNoRows {
        return response, ErrUserNotFound
    }
    if err != nil {
        return response, fmt.Errorf("Couldn't query select user id from followee username: %v", err)
    }
    if followeeID == followerID {
        return response, ErrForbiddenFollow
    }
    query = "SELECT EXISTS (SELECT 1 FROM follows WHERE follower_id = $1 AND followee_id = $2)"
    if err = tx.QueryRowContext(ctx, query, followerID, followeeID).Scan(&response.Following); err != nil {
        return response, fmt.Errorf("Couldn't query select exists due to: %v", err)
    }
    if response.Following {
        query = "DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2"
        if _, err = tx.ExecContext(ctx, query, followerID, followeeID); err != nil {
            return response, fmt.Errorf("Couldn't delete follow: %v", err)
        }
        query = "UPDATE users SET followees_count = followees_count - 1 WHERE id = $1"
        if _, err = tx.ExecContext(ctx, query, followerID); err != nil {
            return response, fmt.Errorf("Couldn't update follower followees_count: %v", err)
        }
        query = "UPDATE users SET followers_count = followers_count - 1 WHERE id = $1 RETURNING followers_count"
        if err = tx.QueryRowContext(ctx, query, followeeID).Scan(&response.FollowersCount); err != nil {
            return response, fmt.Errorf("Couldn't update followee followers count: %v", err)
        }
    } else {
        query = "INSERT INTO follows (follower_id, followee_id) VALUES ($1, $2)"
        if _, err = tx.ExecContext(ctx, query, followerID, followeeID); err != nil {
            return response, fmt.Errorf("Couldn't insert follow: %v", err)
        }
        query = "UPDATE users SET followees_count = followees_count + 1 where id = $1"
        if _, err = tx.ExecContext(ctx, query, followerID); err != nil {
            return response, fmt.Errorf("couldn't update follower followees count: %v", err)
        }
        query = "UPDATE users SET followers_count = followers_count + 1 where id = $1 RETURNING followers_count"
        if err = tx.QueryRowContext(ctx, query, followeeID).Scan(&response.FollowersCount); err != nil {
            return response, fmt.Errorf("Couldn't update followee followers count: %v", err)
        }

    }
    if err = tx.Commit(); err != nil {
        return response, fmt.Errorf("Couldnt commit toggle follow: %v", err)
    }
    response.Following = !response.Following
    if response.Following {
        // TODO: notify following
    }
    return response, nil
}
