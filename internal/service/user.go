package service

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "image"
    "image/jpeg"
    "image/png"
    "io"
    "log"
    "os"
    "path"
    "regexp"
    "strings"

    "github.com/disintegration/imaging"
    gonanoid "github.com/matoous/go-nanoid"
)

var (
    rxEmail    = regexp.MustCompile("^[^\\s@]+@[^\\s@]+\\.[^\\s@]+$")
    rxUsername = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9_-]{0,17}$")
    avatarsDir = path.Join("public", "users", "avatars")
)

var (

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
    //ErrUnsupportedAvatarFormat is used to indicate that uploaded avatar has invalid format.
    ErrUnsupportedAvatarFormat = errors.New("only png and jpeg are allowed as avatars format")
)

//MaxAvatarBytes to read
const MaxAvatarBytes = 5 << 20 // 5 MB
// User Model.
type User struct {
    ID        int64   `json:"id,omitempty"`
    Username  string  `json:"username"`
    AvatarURL *string `json:"avatarUrl,omitempty"`
}

// UserProfile model.
type UserProfile struct {
    User
    Email          string `json:"email,omitempty"`
    FollowersCount int    `json:"followers_count"`
    FolloweesCount int    `json:"followees_count"`
    Me             bool   `json:"me"`
    Following      bool   `json:"following"`
    Followeed      bool   `json:"followeed"`
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

//UpdateAvatar of the authenticated user returning the new avatar url.
func (s *Service) UpdateAvatar(ctx context.Context, r io.Reader) (string, error) {
    uid, ok := ctx.Value(KeyAuthUserID).(int64)
    if !ok {
        return "", ErrUnauthenticated
    }
    r = io.LimitReader(r, MaxAvatarBytes)
    img, format, err := image.Decode(r)
    if err != nil {
        return "", fmt.Errorf("Couldn't read avatar: %v", err)
    }
    if format != "png" && format != "jpeg" {
        return "", ErrUnsupportedAvatarFormat
    }
    avatar, err := gonanoid.Nanoid()
    if err != nil {
        return "", fmt.Errorf("Couldn't generate avatar filename: %v", err)
    }
    if format == "png" {
        avatar += ".png"
    } else {
        avatar += ".jpg"
    }
    avatar += fmt.Sprintf(".%s", format)
    avatarPath := path.Join(avatarsDir, avatar)
    f, err := os.Create(avatarPath)
    if err != nil {
        return "", fmt.Errorf("Couldn't create avatar: %v", err)
    }
    defer f.Close()
    img = imaging.Fill(img, 400, 400, imaging.Center, imaging.CatmullRom)
    if format == "png" {
        err = png.Encode(f, img)
    } else {
        err = jpeg.Encode(f, img, nil)
    }
    if err != nil {
        return "", fmt.Errorf("couldn't write avatar to disk: %v", err)
    }
    var oldAvatar sql.NullString
    if err = s.db.QueryRowContext(ctx, `
        UPDATE users SET avatar = $1 WHERE id = $2
        RETURNING (SELECT avatar FROM users where id = $2) AS old_avatar
    `, avatar, uid).Scan(&oldAvatar); err != nil {
        defer os.Remove(avatarPath)
        return "", fmt.Errorf("couldn't update avatar: %v", err)
    }
    if oldAvatar.Valid {
        defer os.Remove(path.Join(avatarsDir, oldAvatar.String))
    }
    return s.origin + "/public/avatars/users/" + avatar, nil
}

//User selects on user from the database with given username.
func (s *Service) User(ctx context.Context, username string) (UserProfile, error) {
    var u UserProfile
    username = strings.TrimSpace(username)
    if !rxUsername.MatchString(username) {
        return u, ErrInvalidUsername
    }
    uid, auth := ctx.Value(KeyAuthUserID).(int64)
    args := []interface{}{username}
    var avatar sql.NullString
    dest := []interface{}{&u.ID, &u.Email, &avatar, &u.FollowersCount, &u.FolloweesCount}
    query := "SELECT id, email, followers_count, followees_count "
    if auth {
        query += ", " +
            "followers.follower_id IS NOT NULL AS following, " +
            "followees.followee_id IS NOT NULL AS followeed "
        dest = append(dest, &u.Following, &u.Followeed)
    }
    query += "FROM users "
    if auth {
        query += "LEFT JOIN follows as followers on followers.follower_id = $2 AND followers.followee_id = users.id " +
            "LEFT JOIN follows AS followees ON followees.follower_id = users.id AND followees.followee_id = $2"
        args = append(args, uid)

    }
    query += "where username = $1"
    err := s.db.QueryRowContext(ctx, query, args...).Scan(dest...)

    if err == sql.ErrNoRows {
        return u, ErrUserNotFound
    }

    if err != nil {
        return u, fmt.Errorf("Couldn't select user: %v", err)
    }
    u.Username = username
    u.Me = auth && uid == u.ID
    if !u.Me {
        u.ID = 0
        u.Email = ""
    }
    if avatar.Valid {
        avatarURL := s.origin + "/public/avatars/users/" + avatar.String
        u.AvatarURL = &avatarURL
    }
    return u, nil
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

//Users in asc order with forward pagination, and filtered by username
func (s *Service) Users(ctx context.Context, search string, first int, after string) ([]UserProfile, error) {
    search = strings.TrimSpace(search)
    after = strings.TrimSpace(after)
    first = normalizePageSize(first)
    uid, auth := ctx.Value(KeyAuthUserID).(int64)
    query, args, err := buildQuery(`
        SELECT id, email, username, followers_count, followees_count, avatars
        {{if .auth}}
        , followers.follower_id IS NOT NULL AS following
        , followees.followee_id IS NOT NULL AS followeed
        {{end}}
        FROM users
        {{if .auth}}
        LEFT JOIN follows AS followers ON followers.follower_id = @uid AND followers.followee_id = users.id
        LEFT JOIN follows AS followees ON followees.follower_id = users.id AND followees.followee_id = @uid
        {{end}}
        {{if or .search .after}}WHERE{{end}}
        {{if .search}}username ILIKE '%' || @search || '%'{{end}}
        {{if and .search .after}}AND{{end}}
        {{if .after}}username > @after{{end}}
        ORDER BY username ASC
        LIMIT @first`, map[string]interface{}{
        "auth":   auth,
        "uid":    uid,
        "search": search,
        "first":  first,
        "after":  after,
    })
    if err != nil {
        return nil, fmt.Errorf("couldn't build users sql query: %v", err)
    }
    log.Printf("users query: %s \n args: %v\n", query, args)
    rows, err := s.db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, fmt.Errorf("couldn't query select users: %v", err)
    }
    defer rows.Close()
    uu := make([]UserProfile, 0, first)
    for rows.Next() {
        var u UserProfile
        var avatar sql.NullString
        dest := []interface{}{&u.ID, &u.Email, &u.Username, &avatar, &u.FolloweesCount, &u.FolloweesCount}
        if auth {
            dest = append(dest, &u.Following, &u.Followeed)
        }
        if err = rows.Scan(dest...); err != nil {
            return nil, fmt.Errorf("couldn't scan user: %v", err)
        }
        u.Me = auth && uid == u.ID
        if !u.Me {
            u.ID = 0
            u.Email = ""
        }
        if avatar.Valid {
            avatarURL := s.origin + "/public/avatars/users/" + avatar.String
            u.AvatarURL = &avatarURL
        }
        uu = append(uu, u)
    }
    if err = rows.Err(); err != nil {
        return nil, fmt.Errorf("couldn't iterate user rows: %v", err)
    }
    return uu, nil
}

//Followers in asc order with forward pagination
func (s *Service) Followers(ctx context.Context, username string, first int, after string) ([]UserProfile, error) {
    username = strings.TrimSpace(username)
    if !rxUsername.MatchString(username) {
        return nil, ErrInvalidUsername
    }

    after = strings.TrimSpace(after)
    first = normalizePageSize(first)
    uid, auth := ctx.Value(KeyAuthUserID).(int64)
    query, args, err := buildQuery(`
        SELECT id, email, username, followers_count, followees_count, avatar
        {{if .auth}}
        , followers.follower_id IS NOT NULL AS following
        , followees.followee_id IS NOT NULL AS followeed
        {{end}}
        FROM follows
        INNER JOIN users ON follows.follower_id = users.id
        {{if .auth}}
        LEFT JOIN follows AS followers ON followers.follower_id = @uid AND followers.followee_id = users.id
        LEFT JOIN follows AS followees ON followees.follower_id = users.id AND followees.followee_id = @uid
        {{end}}
        WHERE follows.followee_id = (SELECT id from users where username = @username)
        {{if  .after}}AND username > @after{{end}}
        ORDER BY username ASC
        LIMIT @first`, map[string]interface{}{
        "auth":     auth,
        "uid":      uid,
        "username": username,
        "first":    first,
        "after":    after,
    })
    if err != nil {
        return nil, fmt.Errorf("couldn't build followers sql query: %v", err)
    }
    log.Printf("followers query: %s \n args: %v\n", query, args)
    rows, err := s.db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, fmt.Errorf("couldn't query select followers: %v", err)
    }
    defer rows.Close()
    uu := make([]UserProfile, 0, first)
    for rows.Next() {
        var u UserProfile
        var avatar sql.NullString
        dest := []interface{}{&u.ID, &u.Email, &u.Username, &avatar, &u.FolloweesCount, &u.FolloweesCount}
        if auth {
            dest = append(dest, &u.Following, &u.Followeed)
        }
        if err = rows.Scan(dest...); err != nil {
            return nil, fmt.Errorf("couldn't scan follower: %v", err)
        }
        u.Me = auth && uid == u.ID
        if !u.Me {
            u.ID = 0
            u.Email = ""
        }
        if avatar.Valid {
            avatarURL := s.origin + "/public/avatars/users/" + avatar.String
            u.AvatarURL = &avatarURL
        }
        uu = append(uu, u)
    }
    if err = rows.Err(); err != nil {
        return nil, fmt.Errorf("couldn't iterate followers rows: %v", err)
    }
    return uu, nil
}
func (s *Service) userByID(ctx context.Context, id int64) (User, error) {
    var u User
    var avatar sql.NullString
    query := "SELECT username, avatar from users WHERE id = $1"
    err := s.db.QueryRowContext(ctx, query, id).Scan(&u.Username, &avatar)
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
    u.ID = id
    return u, nil
}

//Followees in asc order with forward pagination
func (s *Service) Followees(ctx context.Context, username string, first int, after string) ([]UserProfile, error) {
    username = strings.TrimSpace(username)
    if !rxUsername.MatchString(username) {
        return nil, ErrInvalidUsername
    }
    after = strings.TrimSpace(after)
    first = normalizePageSize(first)
    uid, auth := ctx.Value(KeyAuthUserID).(int64)
    query, args, err := buildQuery(`
        SELECT id, email, username, followers_count, followees_count, avatar
        {{if .auth}}
        , followers.follower_id IS NOT NULL AS following
        , followees.followee_id IS NOT NULL AS followeed
        {{end}}
        FROM follows
        INNER JOIN users ON follows.followee_id = users.id
        {{if .auth}}
        LEFT JOIN follows AS followers ON followers.follower_id = @uid AND followers.followee_id = users.id
        LEFT JOIN follows AS followees ON followees.follower_id = users.id AND followees.followee_id = @uid
        {{end}}
        WHERE follows.follower_id = (SELECT id from users where username = @username)
        {{if  .after}}AND username > @after{{end}}
        ORDER BY username ASC
        LIMIT @first`, map[string]interface{}{
        "auth":     auth,
        "uid":      uid,
        "username": username,
        "first":    first,
        "after":    after,
    })
    if err != nil {
        return nil, fmt.Errorf("couldn't build followees sql query: %v", err)
    }
    log.Printf("followees query: %s \n args: %v\n", query, args)
    rows, err := s.db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, fmt.Errorf("couldn't query select followees: %v", err)
    }
    defer rows.Close()
    uu := make([]UserProfile, 0, first)
    for rows.Next() {
        var u UserProfile
        var avatar sql.NullString
        dest := []interface{}{&u.ID, &u.Email, &u.Username, &avatar, &u.FolloweesCount, &u.FolloweesCount}
        if auth {
            dest = append(dest, &u.Following, &u.Followeed)
        }
        if err = rows.Scan(dest...); err != nil {
            return nil, fmt.Errorf("couldn't scan followee: %v", err)
        }
        u.Me = auth && uid == u.ID
        if !u.Me {
            u.ID = 0
            u.Email = ""
        }
        if avatar.Valid {
            avatarURL := s.origin + "/public/avatars/users/" + avatar.String
            u.AvatarURL = &avatarURL
        }
        uu = append(uu, u)
    }
    if err = rows.Err(); err != nil {
        return nil, fmt.Errorf("couldn't iterate followee rows: %v", err)
    }
    return uu, nil
}
