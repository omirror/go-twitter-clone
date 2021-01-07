package service

import (
    "context"
    "database/sql"
    "fmt"
    "log"
    "time"

    "github.com/lib/pq"
)

// Notification model.
type Notification struct {
    ID       int64     `json:"id"`
    UserID   int64     `json:"-"`
    Actors   []string  `json:"actors"`
    Type     string    `json:"type"`
    Read     bool      `json:"read"`
    PostID   *int64    `json:"post_id, omitempty"`
    IssuedAt time.Time `json:"issued_at"`
}

// Notifications for the authenticated user in desc order with backward pagination
func (s *Service) Notifications(ctx context.Context, last int, before int64) ([]Notification, error) {
    uid, ok := ctx.Value(KeyAuthUserID).(int64)
    if !ok {
        return nil, ErrUnauthenticated
    }
    last = normalizePageSize(last)
    query, args, err := buildQuery(`
        SELECT id, actors, type, read, issued_at, post_id
        FROM notifications
        WHERE user_id = @uid
        {{if .before}}AND id < @before{{end}}
        ORDER BY issued_at DESC
        LIMIT @last
        `, map[string]interface{}{
        "uid":    uid,
        "before": before,
        "last":   last,
    })
    if err != nil {
        return nil, fmt.Errorf("couldn't build notification sql query: %v", err)
    }
    rows, err := s.db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, fmt.Errorf("couldn't query select notifications: %v", err)
    }
    defer rows.Close()
    notifications := make([]Notification, 0, last)
    for rows.Next() {
        var notification Notification
        if err = rows.Scan(&notification.ID, pq.Array(&notification.Actors), &notification.Type, &notification.Read, &notification.IssuedAt, &notification.PostID); err != nil {
            return nil, fmt.Errorf("Couldn't scan notification: %v", err)
        }
        notifications = append(notifications, notification)
    }
    if err = rows.Err(); err != nil {
        return nil, fmt.Errorf("Couldn't iterate over notification rows:%v", err)
    }
    return notifications, nil
}
func (s *Service) MarkNotificationAsRead(ctx context.Context, notificationID int64) error {
    uid, ok := ctx.Value(KeyAuthUserID).(int64)
    if !ok {
        return ErrUnauthenticated
    }
    query := "UPDATE notifications SET read = true WHERE id = $1 AND user_id = $2"
    if _, err := s.db.Exec(query, notificationID, uid); err != nil {
        return fmt.Errorf("Couldn't update and mark notification as read: %v", err)
    }
    return nil
}
func (s *Service) MarkNotificationsAsRead(ctx context.Context) error {
    uid, ok := ctx.Value(KeyAuthUserID).(int64)
    if !ok {
        return ErrUnauthenticated
    }
    query := "UPDATE notifications SET read = true WHERE user_id = $1"
    if _, err := s.db.Exec(query, uid); err != nil {
        return fmt.Errorf("Couldn't update and mark notification as read: %v", err)
    }
    return nil
}
func (s *Service) notifyFollower(followerID, followeeID int64) {
    tx, err := s.db.Begin()
    if err != nil {
        log.Printf("Couldn't begin tx: %v\n", err)
        return
    }
    defer tx.Rollback()
    var actor string
    query := "SELECT username from users WHERE id = $1"
    if err = tx.QueryRow(query, followerID).Scan(&actor); err != nil {
        log.Printf("couldn't query select follow notification actor: %v\n", err)
        return
    }
    var notified bool
    query = `SELECT EXISTS (
        SELECT 1 FROM notifications
        WHERE user_id = $1
            AND actors @> ARRAY[$2]::varchar[]
            AND type = 'follow'
    )`
    if err = tx.QueryRow(query, followeeID, actor).Scan(&notified); err != nil {
        log.Printf("couldn't query select follow notification existence: %v\n", err)
        return
    }
    if notified {
        return
    }
    var notificationID int64
    query = "SELECT id from notifications WHERE user_id = $1 AND type = 'follow' AND read = false"
    err = tx.QueryRow(query, followeeID).Scan(&notificationID)
    if err != nil && err != sql.ErrNoRows {
        log.Printf("couldn't query select unread follow notification: %v\n", err)
        return

    }
    var notification Notification
    if err == sql.ErrNoRows {
        actors := []string{actor}
        query = "INSERT INTO notifications (user_id, actors, type) VALUES ($1, $2, 'follow') RETURNING id, issued_at"
        if err = tx.QueryRow(query, followeeID, pq.Array(actors)).Scan(&notification.ID, &notification.IssuedAt); err != nil {
            log.Printf("Couldn't insert follow notification: %v\n", err)
            return
        }
        notification.Actors = actors
    } else {
        query = `
            UPDATE notifications SET actors = array_prepend($1, notifications.actors), issued_at = now() where id = $2 RETURNING actors, issued_at
        `
        if err = tx.QueryRow(query, actor, notificationID).Scan(pq.Array(&notification.Actors), &notification.IssuedAt); err != nil {
            log.Printf("Couldn't update  follow notification: %v\n", err)
            return
        }
        notification.ID = notificationID
    }
    notification.UserID = followeeID
    notification.Type = "follow"
    if err = tx.Commit(); err != nil {
        log.Printf("Couldn't commit notification: %v\n", err)
        return
    }
    // TODO: broadcast follow notification.
}
func (s *Service) notifyComment(c Comment) {
    actor := c.User.Username
    
    rows, err := s.db.Query(`
        INSERT INTO notifications (user_id, actors, type, post_id)
        SELECT user_id, $1, 'comment', $2 FROM post_subscriptions
        WHERE post_subscriptions.user_id != $3
            AND post_subscriptions.post_id = $2
        ON CONFLICT (user_id, type, post_id, read) DO UPDATE SET
            actors = array_prepend($4, array_remove(notifications.actors, $4)),
            issued_at = now()
        RETURNING id, user_id, actors, issued_at`,
        pq.Array([]string{actor}),
        c.PostID,
        c.UserID,
        actor,
    )
    if err != nil {
        log.Printf("couldn't insert notification with comment: %v", err)
        return
    }
    defer rows.Close()
    notifications := make([]Notification, 0)
    for rows.Next() {
        var notification Notification
        if err = rows.Scan(&notification.ID, &notification.UserID, pq.Array(&notification.Actors), &notification.IssuedAt); err != nil {
            log.Printf("Couldn't scan comment notification: %v", err)
            return
        }
        notification.Type = "comment"
        notification.PostID = &c.PostID
        notifications = append(notifications, notification)
    }
    if err = rows.Err(); err != nil {
        log.Printf("Couldn't iterate over comment notification rows: %v\n", err)
        return
    }
    // TODO: broadcast comment notifications.
}
