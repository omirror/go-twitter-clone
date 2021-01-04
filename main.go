package main

import (
    "database/sql"
    "fmt"
    "log"
    "net/http"

    "github.com/hako/branca"
    _ "github.com/jackc/pgx/stdlib"
    "github.com/secmohammed/go-twitter/internal/handler"
    "github.com/secmohammed/go-twitter/internal/service"
)

const (
    databaseURL = "postgresql://mohammed:root@127.0.0.1:5432/go_twitter?sslmode=disable"
    port        = 3000
)

func main() {
    db, err := sql.Open("pgx", databaseURL)
    if err != nil {
        log.Fatalf("couldn't open db connection: %v \n", err)
        return
    }
    defer db.Close()
    if err = db.Ping(); err != nil {
        log.Fatalf("couldn't ping to db: %v \n", err)
        return
    }
    codec := branca.NewBranca("supersecretkeyyoushouldnotcommit")
    codec.SetTTL(uint32(service.TokenTTL.Seconds()))
    s := service.New(db, codec)
    h := handler.New(s)
    addr := fmt.Sprintf(":%d", port)
    log.Printf("accepting connections on port %d\n", port)
    if err = http.ListenAndServe(addr, h); err != nil {
        log.Fatalf("Couldn't start server: %v\n", err)
    }
}
