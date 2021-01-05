package handler

import (
    "encoding/json"
    "net/http"

    "github.com/secmohammed/go-twitter/internal/service"
)

type createPostInput struct {
    Content   string
    SpoilerOf *string
    NSFW      bool
}

func (h *handler) createPost(w http.ResponseWriter, r *http.Request) {
    var input createPostInput
    defer r.Body.Close()
    if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    ti, err := h.CreatePost(r.Context(), input.Content, input.SpoilerOf, input.NSFW)
    if err == service.ErrUnauthenticated {
        http.Error(w, err.Error(), http.StatusUnauthorized)
        return
    }
    if err == service.ErrInvalidContent || err == service.ErrInvalidSpoiler {
        http.Error(w, err.Error(), http.StatusUnprocessableEntity)
        return
    }
    if err != nil {
        respondError(w, err)
        return
    }
    respond(w, ti, http.StatusCreated)
}
