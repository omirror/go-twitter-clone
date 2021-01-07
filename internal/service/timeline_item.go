package service

import (
    "context"
    "database/sql"
    "fmt"
)

//TimelineItem model.
type TimelineItem struct {
    ID     int64 `json:"id"`
    UserID int64 `json:"-"`
    PostID int64 `json:"-"`
    Post   Post  `json:"post"`
    User   *User `json:"user,omitempty"`
}

//Timeline is used to show the timeline of the authenticated user.
func (s *Service) Timeline(ctx context.Context, last int, before int64) ([]TimelineItem, error) {
    uid, ok := ctx.Value(KeyAuthUserID).(int64)
    if !ok {
        return nil, ErrUnauthenticated
    }
    last = normalizePageSize(last)
    query, args, err := buildQuery(`
        SELECT timeline.id, posts.id, content, spoiler_of, nsfw, likes_count, created_at, comments_count
        , posts.user_id = @uid AS mine
        , likes.user_id IS NOT NULL AS liked
        , subscriptions.user_id IS NOT NULL AS subscribed
        , users.username, users.avatar
        FROM timeline
        INNER JOIN posts on timeline.post_id = posts.id
        INNER JOIN users on posts.user_id = users.id
        LEFT JOIN post_likes AS likes
        ON likes.user_id = @uid AND likes.post_id = posts.id
        LEFT JOIN post_subscriptions AS subscriptions
 			ON subscriptions.user_id = @uid AND subscriptions.post_id = posts.id
        WHERE timeline.user_id = @uid
        {{if .before}} AND posts.id < @before{{end}}
        ORDER BY created_at DESC
        LIMIT @last
    `, map[string]interface{}{
        "uid":    uid,
        "last":   last,
        "before": before,
    })
    if err != nil {
        return nil, fmt.Errorf("Couldn't build timeline query: %v", err)
    }
    rows, err := s.db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, fmt.Errorf("Couldn't query select timeline: %v", err)
    }
    defer rows.Close()
    tt := make([]TimelineItem, 0, last)
    for rows.Next() {
        var ti TimelineItem
        var u User
        var avatar sql.NullString
        dest := []interface{}{
            &ti.ID,
            &ti.Post.ID,
            &ti.Post.Content,
            &ti.Post.SpoilerOf,
            &ti.Post.NSFW,
            &ti.Post.LikesCount,
            &ti.Post.CreatedAt,
            &ti.Post.Mine,
            &ti.Post.Liked,
            &ti.Post.Subscribed,
            &u.Username,
            &avatar,
            &ti.Post.CommentsCount,
        }
        if err = rows.Scan(dest...); err != nil {
            return nil, fmt.Errorf("couldn't scan post: %v", err)
        }
        if avatar.Valid {
            avatarURL := s.origin + "/public/avatars/users/" + avatar.String
            u.AvatarURL = &avatarURL
        }
        ti.Post.User = &u
        tt = append(tt, ti)
    }
    if err = rows.Err(); err != nil {
        return nil, fmt.Errorf("couldn't iterate posts rows: %v", err)
    }

    return tt, nil
}
