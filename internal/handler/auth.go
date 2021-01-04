package handler

import (
    "context"
    "encoding/json"
    "net/http"
    "strings"

    "github.com/secmohammed/go-twitter/internal/service"
)

type loginInput struct {
    Email string
}

func (h *handler) login(w http.ResponseWriter, r *http.Request) {
    var loginInput loginInput
    if err := json.NewDecoder(r.Body).Decode(&loginInput); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    response, err := h.Login(r.Context(), loginInput.Email)
    if err == service.ErrInvalidEmail {
        http.Error(w, err.Error(), http.StatusUnprocessableEntity)
        return
    }
    if err == service.ErrUserNotFound {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }
    if err != nil {
        respondError(w, err)
        return
    }
    respond(w, response, http.StatusOK)
}

func (h *handler) authUser(w http.ResponseWriter, r *http.Request) {
    u, err := h.AuthUser(r.Context())
    if err == service.ErrUnauthenticated {
        http.Error(w, err.Error(), http.StatusUnauthorized)
        return
    }
    if err == service.ErrUserNotFound {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }
    if err != nil {
        respondError(w, err)
        return
    }
    respond(w, u, http.StatusOK)
}
func (h *handler) withAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        a := r.Header.Get("Authorization")
        if !strings.HasPrefix(a, "Bearer ") {
            next.ServeHTTP(w, r)
            return
        }
        token := a[7:]
        uid, err := h.AuthUserID(token)
        if err != nil {
            http.Error(w, err.Error(), http.StatusUnauthorized)
            return
        }
        ctx := r.Context()
        ctx = context.WithValue(ctx, service.KeyAuthUserID, uid)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
