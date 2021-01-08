package main

import (
    "database/sql"
    "log"
    "net/http"
    "os"
    "strconv"

    "github.com/joho/godotenv"
    _ "github.com/lib/pq"
    "github.com/secmohammed/go-twitter/internal/handler"
    "github.com/secmohammed/go-twitter/internal/service"
)

func main() {
    godotenv.Load()
    var (
        port         = env("PORT", "3000")
        origin       = env("ORIGIN", "http://localhost:"+port)
        databaseURL  = env("DATABASE_URL", "postgresql://mohammed:root@127.0.0.1:5432/go_twitter?sslmode=disable")
        secretKey    = env("SECRET_KEY", "supersecretkeyyoushouldnotcommit")
        smtpHost     = env("SMTP_HOST", "smtp.mailtrap.io")
        smtpPort     = intEnv("SMTP_PORT", 25)
        smtpUsername = mustEnv("SMTP_USERNAME")
        smtpPassword = mustEnv("SMTP_PASSWORD")
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
    s := service.New(service.Config{
        DB:           db,
        Origin:       origin,
        SecretKey:    secretKey,
        SMTPHost:     smtpHost,
        SMTPPort:     smtpPort,
        SMTPPassword: smtpPassword,
        SMTPUsername: smtpUsername,
    })
    h := handler.New(s)
    if err = http.ListenAndServe(":"+port, h); err != nil {
        log.Fatalf("Couldn't start server: %v\n", err)
    }
}
func env(key, fallbackValue string) string {
    s, ok := os.LookupEnv(key)
    if !ok {
        return fallbackValue
    }
    return s
}
func mustEnv(key string) string {
    s, ok := os.LookupEnv(key)
    if !ok {
        log.Fatalf("%s missing on env variables \n", key)
        return ""
    }
    return s
}
func intEnv(key string, fallbackValue int) int {
    s, ok := os.LookupEnv(key)
    if !ok {
        return fallbackValue
    }
    i, err := strconv.Atoi(s)
    if err != nil {
        return fallbackValue
    }
    return i
}
