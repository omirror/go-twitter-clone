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
type sendMagicLinkInput struct {
    Email       string
    RedirectURI string
}

func (h *handler) sendMagicLink(w http.ResponseWriter, r *http.Request) {
    var sendMagicLinkInput sendMagicLinkInput
    if err := json.NewDecoder(r.Body).Decode(&sendMagicLinkInput); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    err := h.SendMagicLink(r.Context(), sendMagicLinkInput.Email, sendMagicLinkInput.RedirectURI)
    if err == service.ErrInvalidEmail || err == service.ErrInvalidRedirectURI {
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
    w.WriteHeader(http.StatusNoContent)

}
func (h *handler) authRedirect(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query()
    uri, err := h.AuthURI(r.Context(), q.Get("verification_code"), q.Get("redirect_uri"))
    if err == service.ErrInvalidVerificationCode || err == service.ErrInvalidRedirectURI {
        http.Error(w, err.Error(), http.StatusUnprocessableEntity)
        return

    }
    if err == service.ErrVerificationCodeNotFound {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }
    if err == service.ErrVerificationCodeExpired {
        http.Error(w, err.Error(), http.StatusGone)
        return
    }

    if err != nil {
        respondError(w, err)
        return

    }
    http.Redirect(w, r, uri, http.StatusFound)
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
