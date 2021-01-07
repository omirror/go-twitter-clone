package handler

import (
    "net/http"
    "strconv"

    "github.com/matryer/way"
    "github.com/secmohammed/go-twitter/internal/service"
)

func (h *handler) notifications(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query()
    last, _ := strconv.Atoi(q.Get("last"))
    before, _ := strconv.ParseInt(q.Get("before"), 10, 64)
    notifications, err := h.Notifications(r.Context(), last, before)
    if err == service.ErrUnauthenticated {
        http.Error(w, err.Error(), http.StatusUnauthorized)
        return
    }
    if err != nil {
        respondError(w, err)
        return
    }
    respond(w, notifications, http.StatusOK)
}
func (h *handler) markNotificationAsRead(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    notificationID, _ := strconv.ParseInt(way.Param(ctx, "notification_id"), 10, 64)
    err := h.MarkNotificationAsRead(ctx, notificationID)
    if err == service.ErrUnauthenticated {
        http.Error(w, err.Error(), http.StatusUnauthorized)
        return
    }
    if err != nil {
        respondError(w, err)
        return
    }
    w.WriteHeader(http.StatusNoContent)

}
func (h *handler) markAllNotificationsAsRead(w http.ResponseWriter, r *http.Request) {
    err := h.MarkNotificationsAsRead(r.Context())
    if err == service.ErrUnauthenticated {
        http.Error(w, err.Error(), http.StatusUnauthorized)
        return
    }
    if err != nil {
        respondError(w, err)
        return
    }
    w.WriteHeader(http.StatusNoContent)

}
