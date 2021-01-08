package service

import (
    "bytes"
    "context"
    "database/sql"
    "errors"
    "fmt"
    "log"
    "net/url"
    "regexp"
    "strconv"
    "strings"
    "text/template"
    "time"
)

// KeyAuthUserID is used to identify the auth_user_key

const KeyAuthUserID key = "auth_user_id"
const (
    tokenTTL            = time.Hour * 24 * 14
    verificationCodeTTL = time.Minute * 15
)

var magicLinkTmpl *template.Template
var rxUUID = regexp.MustCompile("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")

var (
    //ErrUnauthenticated is used when the user isn't authenticated, and trying to access something requires authentication.
    ErrUnauthenticated = errors.New("unauthenticated")
    //ErrInvalidRedirectURI is used when the url is invalid.
    ErrInvalidRedirectURI = errors.New("Invalid url redirection")
    //ErrInvalidVerificationCode is used to denote that the verification code pattern is incorrect.
    ErrInvalidVerificationCode = errors.New("Invalid verification code")
    //ErrVerificationCodeNotFound is used to denote that verification code isn't found at db.
    ErrVerificationCodeNotFound = errors.New("verification code not found")
    //ErrVerificationCodeExpired is used to denote that verification code is already expired.
    ErrVerificationCodeExpired = errors.New("Verification code is already expired")
)

type key string

// LoginOutput is the login response.
type LoginOutput struct {
    Token     string    `json:"token"`
    ExpiresAt time.Time `json:"expires_at"`
    User      User      `json:"user"`
}

//AuthUserID from token
func (s *Service) AuthUserID(token string) (int64, error) {
    str, err := s.codec.DecodeToString(token)
    if err != nil {
        return 0, fmt.Errorf("could not decode token: %v", err)
    }
    i, err := strconv.ParseInt(str, 10, 64)
    if err != nil {
        return 0, fmt.Errorf("Couldn't parse auth user id from token: %v", err)
    }
    return i, nil
}

//SendMagicLink is used to passwordless authentication.
func (s *Service) SendMagicLink(ctx context.Context, email, redirectURI string) error {
    email = strings.TrimSpace(email)
    if !rxEmail.MatchString(email) {
        return ErrInvalidEmail
    }
    uri, err := url.ParseRequestURI(redirectURI)
    if err != nil {
        return ErrInvalidRedirectURI
    }
    var verificationCode string
    err = s.db.QueryRowContext(ctx, `INSERT INTO verification_codes (user_id)  (
        SELECT id FROM users WHERE email = $1
    ) RETURNING id`, email).Scan(&verificationCode)
    if isForeignKeyViolation(err) {
        return ErrUserNotFound
    }
    if err != nil {
        return fmt.Errorf("Couldn't insert verification code: %v", err)
    }
    magicLink, _ := url.Parse(s.origin)
    magicLink.Path = "/api/auth_redirect"
    q := magicLink.Query()
    q.Set("verification_code", verificationCode)
    q.Set("redirect_uri", uri.String())
    magicLink.RawQuery = q.Encode()
    if magicLinkTmpl == nil {
        magicLinkTmpl, err = template.ParseFiles("mail/template/magicLink.html")
        if err != nil {
            return fmt.Errorf("Couldn't parse magic link mail template:%v", err)
        }
    }
    var mail bytes.Buffer
    if err = magicLinkTmpl.Execute(&mail, map[string]interface{}{
        "MagicLink": magicLink.String(),
        "Minutes":   int(verificationCodeTTL.Minutes()),
    }); err != nil {
        return fmt.Errorf("Couldn't execute magic link mail template :%v", err)
    }
    if err = s.sendMail(email, "Magic Link", mail.String()); err != nil {
        return fmt.Errorf("Couldn't send magic link: %v", err)
    }
    return nil
}

// AuthURI to be redirected to and complete the login process.
func (s *Service) AuthURI(ctx context.Context, verificationCode, redirectURI string) (string, error) {
    verificationCode = strings.TrimSpace(verificationCode)
    if !rxUUID.MatchString(verificationCode) {
        return "", ErrInvalidVerificationCode
    }
    uri, err := url.ParseRequestURI(redirectURI)
    if err != nil {
        return "", ErrInvalidRedirectURI
    }
    var uuid int64
    var ts time.Time
    err = s.db.QueryRowContext(ctx, `
        DELETE FROM verification_codes WHERE id = $1 RETURNING user_id, created_at`, verificationCode).Scan(&uuid, &ts)
    if err == sql.ErrNoRows {
        return "", ErrVerificationCodeNotFound
    }
    if err != nil {
        return "", fmt.Errorf("Couldn't delete verification code: %v", err)
    }
    if ts.Add(verificationCodeTTL).Before(time.Now()) {
        return "", ErrVerificationCodeExpired
    }
    token, err := s.codec.EncodeToString(strconv.FormatInt(uuid, 10))
    if err != nil {
        return "", fmt.Errorf("Couldn't create token: %v", err)
    }
    exp, err := time.Now().Add(verificationCodeTTL).MarshalText()
    if err != nil {
        return "", fmt.Errorf("Couldn't marshal token ttl: %v", err)
    }
    f := url.Values{}
    f.Set("token", token)
    f.Set("expires_at", string(exp))
    uri.Fragment = f.Encode()
    return uri.String(), nil
}

// Login user
func (s *Service) Login(ctx context.Context, email string) (LoginOutput, error) {
    var response LoginOutput
    email = strings.TrimSpace(email)
    if !rxEmail.MatchString(email) {
        return response, ErrInvalidEmail
    }
    var avatar sql.NullString
    query := "SELECT id, username, avatar FROM users where email = $1"
    err := s.db.QueryRowContext(ctx, query, email).Scan(&response.User.ID, &response.User.Username, &avatar)
    if err == sql.ErrNoRows {
        return response, ErrUserNotFound
    }
    if err != nil {
        return response, fmt.Errorf("could not query select user: %v", err)
    }
    if avatar.Valid {
        avatarURL := s.origin + "/avatars/users/" + avatar.String
        response.User.AvatarURL = &avatarURL
    }
    response.Token, err = s.codec.EncodeToString(strconv.FormatInt(response.User.ID, 10))
    if err != nil {
        return response, fmt.Errorf("couldn't create token: %v", err)
    }
    response.ExpiresAt = time.Now().Add(tokenTTL)
    return response, nil
}

//AuthUser from context
func (s *Service) AuthUser(ctx context.Context) (User, error) {
    var u User
    uid, ok := ctx.Value(KeyAuthUserID).(int64)
    if !ok {
        return u, ErrUnauthenticated
    }
    return s.userByID(ctx, uid)
}

func (s *Service) deleteExpiredVerificationCodes(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case <-time.After(time.Hour * 24):
            if _, err := s.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM verification_codes WHERE created_at < now() - INTERVAL '%dm'`, int(verificationCodeTTL.Minutes()))); err != nil {
                log.Printf("couldn't delete expired verification codes: %v", err)
            }
        }
    }
}
