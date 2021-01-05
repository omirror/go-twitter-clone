package service

import (
    "context"
    "errors"
    "fmt"
    "log"
    "strings"
    "time"

    "github.com/sanity-io/litter"
)

var (
    // ErrInvalidContent is used to indicate that content is invalid.
    ErrInvalidContent = errors.New("invalid content")
    //ErrInvalidSpoiler is used to indicate that spoiler is invalid.
    ErrInvalidSpoiler = errors.New("invalid spoiler")
)

// Post model.
type Post struct {
    ID        int64     `json:"id"`
    UserID    int64     `json:"-"`
    Content   string    `json:"content"`
    SpoilerOf *string   `json:"spoiler_of"` // it could be null, so it's a pointer.
    NSFW      bool      `json:"nsfw"`
    CreatedAt time.Time `json:"created_at"`
    User      *User     `json:"user,omitempty"`
    Mine      bool      `json:"mine"`
}

//CreatePost publishes a post to the user timeline and fans out it to his followers.
func (s *Service) CreatePost(ctx context.Context, content string, spoilerOf *string, nsfw bool) (TimelineItem, error) {
    var ti TimelineItem
    uid, ok := ctx.Value(KeyAuthUserID).(int64)
    if !ok {
        return ti, ErrUnauthenticated
    }
    content = strings.TrimSpace(content)
    if content == "" || len([]rune(content)) > 480 {
        return ti, ErrInvalidContent
    }
    if spoilerOf != nil {
        *spoilerOf = strings.TrimSpace(*spoilerOf)
        if *spoilerOf == "" || len([]rune(*spoilerOf)) > 64 {
            return ti, ErrInvalidSpoiler
        }
    }
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return ti, fmt.Errorf("Couldn't begin transaction: %v", err)
    }
    defer tx.Rollback()
    query := "INSERT INTO posts (user_id, content, spoiler_of, nsfw) VALUES ($1, $2, $3, $4) RETURNING id, created_at"
    if err = tx.QueryRowContext(ctx, query, uid, content, spoilerOf, nsfw).Scan(&ti.Post.ID, &ti.Post.CreatedAt); err != nil {
        return ti, fmt.Errorf("couldn't insert post: %v", err)
    }
    ti.Post.UserID = uid
    ti.Post.Content = content
    ti.Post.SpoilerOf = spoilerOf
    ti.Post.NSFW = nsfw
    ti.Post.Mine = true
    query = "INSERT INTO timeline (user_id, post_id) VALUES ($1, $2) RETURNING id"
    if err = tx.QueryRowContext(ctx, query, uid, ti.Post.ID).Scan(&ti.ID); err != nil {
        return ti, fmt.Errorf("couldn't insert timeline item: %v", err)
    }
    ti.UserID = uid
    ti.PostID = ti.Post.ID
    if err = tx.Commit(); err != nil {
        return ti, fmt.Errorf("Couldn't commit to create post: %v", err)
    }
    go func(p Post) {
        u, err := s.userByID(context.Background(), p.UserID)
        if err != nil {
            log.Printf("couldn't get post user: %v\n", err)
            return
        }
        p.User = &u
        p.Mine = false
        tt, err := s.fanoutPost(p)
        if err != nil {
            log.Printf("couldn't fanout post: %v\n", err)
            return
        }
        for _, ti = range tt {
            log.Println(litter.Sdump(ti))
            // TODO: broadcast timeline items
        }
    }(ti.Post)
    return ti, nil
}
func (s *Service) fanoutPost(p Post) ([]TimelineItem, error) {
    query := "INSERT INTO timeline (user_id, post_id) SELECT follower_id, $1 FROM follows WHERE followee_id = $2 RETURNING id, user_id"
    rows, err := s.db.Query(query, p.ID, p.UserID)
    if err != nil {
        return nil, fmt.Errorf("couldn't insert timeline: %v", err)
    }
    defer rows.Close()
    tt := []TimelineItem{}
    for rows.Next() {
        var ti TimelineItem
        if err = rows.Scan(&ti.ID, &ti.UserID); err != nil {
            return nil, fmt.Errorf("Couldn't scan timeline item: %v", err)
        }
        ti.PostID = p.ID
        ti.Post = p
        tt = append(tt, ti)
    }
    if err = rows.Err(); err != nil {
        return nil, fmt.Errorf("couldn't iterate over timelines: %v", err)
    }
    return tt, nil
}
