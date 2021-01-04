package handler

import (
    "encoding/json"
    "net/http"

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
