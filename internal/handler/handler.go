package handler

import (
    "net/http"

    "github.com/matryer/way"
    "github.com/secmohammed/go-twitter/internal/service"
)

type handler struct {
    *service.Service
}

// New  creates an http.Handler with predefined routing.
func New(s *service.Service) http.Handler {
    h := &handler{s}
    api := way.NewRouter()
    api.HandleFunc("POST", "/login", h.login)
    api.HandleFunc("GET", "/user", h.authUser)
    api.HandleFunc("POST", "/users/:username/toggle_follow", h.toggleFollow)
    api.HandleFunc("POST", "/users", h.createUser)
    r := way.NewRouter()
    r.Handle("*", "/api...", http.StripPrefix("/api", h.withAuth(api)))
    return r
}
