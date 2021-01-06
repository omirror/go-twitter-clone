package main

import (
    "database/sql"
    "log"
    "net/http"
    "os"

    "github.com/hako/branca"
    _ "github.com/lib/pq"
    "github.com/secmohammed/go-twitter/internal/handler"
    "github.com/secmohammed/go-twitter/internal/service"
)

func main() {
    var (
        port        = env("PORT", "3000")
        origin      = env("ORIGIN", "http://localhost:"+port)
        databaseURL = env("DATABASE_URL", "postgresql://mohammed:root@127.0.0.1:5432/go_twitter?sslmode=disable")
        brancaKey   = env("BRANCA_KEY", "supersecretkeyyoushouldnotcommit")
    )
    db, err := sql.Open("postgres", databaseURL)
    if err != nil {
        log.Fatalf("couldn't open db connection: %v \n", err)
        return
    }
    defer db.Close()
    if err = db.Ping(); err != nil {
        log.Fatalf("couldn't ping to db: %v \n", err)
        return
    }
    codec := branca.NewBranca(brancaKey)
    codec.SetTTL(uint32(service.TokenTTL.Seconds()))
    s := service.New(db, codec, origin)
    h := handler.New(s)
    if err = http.ListenAndServe(":"+port, h); err != nil {
        log.Fatalf("Couldn't start server: %v\n", err)
    }
}
func env(key, fallbackValue string) string {
    s := os.Getenv(key)
    if s == "" {
        return fallbackValue
    }
    return s
}
