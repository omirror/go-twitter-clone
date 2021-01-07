package service

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "log"
    "strings"
    "time"
)

var ErrCommentNotFound = errors.New("Couldn't find comment with this id")

// Comment moddel.
type Comment struct {
    ID         int64     `json:"id"`
    UserID     int64     `json:"-"`
    PostID     int64     `json:"-"`
    Content    string    `json:"content"`
    LikesCount int       `json:"likes_count"`
    CreatedAt  time.Time `json:"created_at"`
    User       *User     `json:"user,omitempty"`
    Post       *Post     `json:"post,omitempty"`
    Mine       bool      `json:"mine"`
    Liked      bool      `json:"liked"`
}

func (s *Service) Comments(ctx context.Context, postID int64, last int, before int64) ([]Comment, error) {
    uid, auth := ctx.Value(KeyAuthUserID).(int64)
    last = normalizePageSize(last)
    query, args, err := buildQuery(`
        SELECT comments.id, content, likes_count, created_at, username, avatar
        {{if .auth}}
        , comments.user_id = @uid AS mine
        , likes.user_id IS NOT NULL AS liked
        {{end}}
        FROM comments
        INNER JOIN users ON comments.user_id = users.id
        {{if .auth}}
        LEFT JOIN comment_likes AS likes ON likes.comment_id = comments.id AND likes.user_id = @uid
        {{end}}
        WHERE comments.post_id = @post_id
        {{if .before}}AND comments.id < @before {{end}}
        ORDER BY created_at DESC
        LIMIT @last
    `, map[string]interface{}{
        "post_id": postID,
        "last":    last,
        "before":  before,
        "uid":     uid,
        "auth":    auth,
    })
    if err != nil {
        return nil, fmt.Errorf("Couldn't build comments query:%v", err)
    }
    rows, err := s.db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, fmt.Errorf("couldn't query comments: %v", err)
    }
    defer rows.Close()
    cc := make([]Comment, 0, last)
    for rows.Next() {
        var c Comment
        var u User
        var avatar sql.NullString
        dest := []interface{}{&c.ID, &c.Content, &c.LikesCount, &c.CreatedAt, &u.Username, &avatar}
        if auth {
            dest = append(dest, &c.Mine, &c.Liked)
        }
        if err = rows.Scan(dest...); err != nil {
            return nil, fmt.Errorf("Couldn't scan comment: %v", err)
        }
        if avatar.Valid {
            avatarURL := s.origin + "/public/avatars/users/" + avatar.String
            u.AvatarURL = &avatarURL
        }
        c.User = &u
        cc = append(cc, c)
    }
    if err = rows.Err(); err != nil {
        return nil, fmt.Errorf("Couldn't iterate comment rows: %v", err)
    }
    return cc, nil
}
func (s *Service) CreateComment(ctx context.Context, postID int64, content string) (Comment, error) {
    var comment Comment
    uid, ok := ctx.Value(KeyAuthUserID).(int64)
    if !ok {
        return comment, ErrUnauthenticated
    }
    content = strings.TrimSpace(content)
    if content == "" || len([]rune(content)) > 480 {
        return comment, ErrInvalidContent
    }
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return comment, fmt.Errorf("Couldn't start transaction:%v", err)
    }
    defer tx.Rollback()
    query := "INSERT INTO comments (post_id, user_id, likes_count, content) VALUES($1, $2, $3, $4) RETURNING id, created_at"
    err = tx.QueryRowContext(ctx, query, postID, uid, 0, content).Scan(&comment.ID, &comment.CreatedAt)
    if isForeignKeyViolation(err) {
        return comment, ErrPostNotFound
    }
    comment.Mine = true
    comment.PostID = postID
    comment.UserID = uid
    comment.Content = content
    comment.LikesCount = 0
    comment.Liked = false
    var subscriptionExists bool
    query = `SELECT EXISTS (
        SELECT 1 from post_subscriptions WHERE user_id = $1 AND post_id = $2    
    )`
    if err = tx.QueryRowContext(ctx, query, uid, postID).Scan(&subscriptionExists); err != nil {
        return comment, fmt.Errorf("Couldn't select exists of post_subscription query :%v", err)
    }
    if !subscriptionExists {
        query = `
            INSERT INTO post_subscriptions (user_id, post_id) VALUES ($1, $2)
        `
        if _, err = tx.ExecContext(ctx, query, uid, postID); err != nil {
            return comment, fmt.Errorf("Couldn't insert post subscription after comment :%v", err)
        }

    }

    query = "UPDATE posts SET comments_count = comments_count + 1 where id = $1"
    if _, err = tx.ExecContext(ctx, query, postID); err != nil {
        return comment, fmt.Errorf("Couldn't update and increment post comments count: %v", err)
    }
    if err := tx.Commit(); err != nil {
        return comment, fmt.Errorf("Couldn't commit creating comment:%v", err)
    }
    go s.commentCreated(comment)
    return comment, nil

}
func (s *Service) commentCreated(c Comment) {
    u, err := s.userByID(context.Background(), c.UserID)
    if err != nil {
        log.Printf("Couldn't fetch comment user: %v", err)
        return
    }
    c.User = &u
    c.Mine = false
    go s.notifyComment(c)
    go s.notifyCommentMention(c)
    // TODO: broadcast comment.
}
func (s *Service) ToggleCommentLike(ctx context.Context, commentID int64) (ToggleLikeResponse, error) {
    var response ToggleLikeResponse
    uid, ok := ctx.Value(KeyAuthUserID).(int64)
    if !ok {
        return response, ErrUnauthenticated
    }
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return response, fmt.Errorf("Couldn't start transaction: %v", err)
    }
    defer tx.Rollback()
    query := `
        SELECT EXISTS (
            SELECT 1 FROM comment_likes WHERE user_id = $1 AND comment_id = $2
        )
    `
    if err := tx.QueryRowContext(ctx, query, uid, commentID).Scan(&response.Liked); err != nil {
        return response, fmt.Errorf("Couldn't find comment to like: %v", err)
    }
    if response.Liked {
        query = "DELETE FROM comment_likes WHERE user_id = $1 AND comment_id = $2"
        if _, err = tx.ExecContext(ctx, query, uid, commentID); err != nil {
            return response, fmt.Errorf("couldn't delete comment: %v", err)
        }
        query = "UPDATE comments SET likes_count = likes_count - 1 where id = $1 RETURNING likes_count"
        if err = tx.QueryRowContext(ctx, query, commentID).Scan(&response.LikesCount); err != nil {
            return response, fmt.Errorf("Couldnt update and decrement comment likes count: %v", err)
        }
    } else {
        query = "INSERT INTO comment_likes (user_id, comment_id) VALUES ($1, $2)"
        _, err = tx.ExecContext(ctx, query, uid, commentID)
        if isForeignKeyViolation(err) {
            return response, ErrCommentNotFound
        }
        if err != nil {
            return response, fmt.Errorf("couldn't insert comment like:%v", err)
        }
        query = "UPDATE comments SET likes_count = likes_count + 1 WHERE id = $1 RETURNING likes_count"
        if err = tx.QueryRowContext(ctx, query, commentID).Scan(&response.LikesCount); err != nil {
            return response, fmt.Errorf("Couldn't update and increment comment likes count: %v", err)
        }
    }
    if err = tx.Commit(); err != nil {
        return response, fmt.Errorf("Couldn't commit to toggle comment like: %v", err)
    }
    response.Liked = !response.Liked
    return response, nil
}
