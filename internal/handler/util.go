package handler

import (
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "log"
    "net/http"
)

var errStreamingUnsupported = errors.New("stremaing unspoorted")

func respond(w http.ResponseWriter, v interface{}, statusCode int) {
    b, err := json.Marshal(v)
    if err != nil {
        respondError(w, fmt.Errorf("couldn't marshal response: %v", err))
        return
    }
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(statusCode)
    w.Write(b)
}
func respondError(w http.ResponseWriter, err error) {
    log.Println(err)
    http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}
func writeSSe(w io.Writer, v interface{}) {
    b, err := json.Marshal(v)
    if err != nil {
        log.Printf("couldn't marshal response: %v", err)
        fmt.Fprintf(w, "error: %v\n'n", err)
        return
    }
    fmt.Fprintf(w, "data: %s\n\n", b)
}
