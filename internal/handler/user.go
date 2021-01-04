package handler

import (
    "encoding/json"
    "net/http"

    "github.com/secmohammed/go-twitter/internal/service"
)

type createUserInput struct {
    Email, Username string
}

func (h *handler) createUser(w http.ResponseWriter, r *http.Request) {
    var createUserInput createUserInput
    defer r.Body.Close()
    if err := json.NewDecoder(r.Body).Decode(&createUserInput); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return

    }
    err := h.CreateUser(r.Context(), createUserInput.Email, createUserInput.Username)
    if err == service.ErrInvalidEmail || err == service.ErrInvalidUsername {
        http.Error(w, err.Error(), http.StatusUnprocessableEntity)
        return
    }
    if err == service.ErrEmailNotUnique || err == service.ErrUsernameNotUnique {
        http.Error(w, err.Error(), http.StatusUnprocessableEntity)
        return
    }
    if err != nil {
        respondError(w, err)
        return
    }
    w.WriteHeader(http.StatusNoContent)
}
