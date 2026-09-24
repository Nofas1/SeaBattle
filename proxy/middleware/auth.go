package mw

import (
    "context"
    "errors"
    "fmt"
    "log/slog"
    "os"
    "sea_battle/proxy/repository"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"
    "golang.org/x/crypto/bcrypt"
)

const jwtSecretEnv = "JWT_SECRET"     // env var with the token signing key
const defaultTokenTTL = time.Hour     // used when a non-positive TTL is passed to New
const issuerName = "sea_battle-proxy" // "iss" claim value

// Authentication, Authorization, Accounting
type AAA struct {
    rep      *repository.Repo
    secret   []byte
    tokenTTL time.Duration
    log      *slog.Logger
}

// New builds the auth service. The signing key is taken from the JWT_SECRET
// environment variable; it must be present and long enough (>= 32 bytes),
// so tokens cannot be forged with a hardcoded key.
func New(tokenTTL time.Duration, log *slog.Logger, rep *repository.Repo) (AAA, error) {
    secret := os.Getenv(jwtSecretEnv)
    if secret == "" {
        return AAA{}, fmt.Errorf("%s environment variable is not set", jwtSecretEnv)
    }
    if len(secret) < 32 {
        return AAA{}, fmt.Errorf("%s must be at least 32 bytes long", jwtSecretEnv)
    }
    if tokenTTL <= 0 {
        tokenTTL = defaultTokenTTL
    }

    return AAA{
        rep:      rep,
        secret:   []byte(secret),
        tokenTTL: tokenTTL,
        log:      log,
    }, nil
}

func (a *AAA) Register(name, password string) error {
    hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        return fmt.Errorf("failed to hash password: %w", err)
    }
    if err := a.rep.RegisterUser(context.Background(), name, string(hashed)); err != nil {
        return fmt.Errorf("failed to register user: %w", err)
    }
    // if _, exists := a.users[name]; !exists {
    //     a.users[name] = password
    // } else {
    //     return errors.New("User already exists")
    // }
    return nil
}

// issueToken creates a signed JWT for the given user.
func (a *AAA) issueToken(name string) (string, error) {
    now := time.Now()
    token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
        Issuer:    issuerName,
        Subject:   name,
        ID:        uuid.NewString(), // jti — unique token id
        IssuedAt:  jwt.NewNumericDate(now),
        NotBefore: jwt.NewNumericDate(now),
        ExpiresAt: jwt.NewNumericDate(now.Add(a.tokenTTL)),
    }).SignedString(a.secret)
    if err != nil {
        return "", fmt.Errorf("failed to sign token: %w", err)
    }
    return token, nil
}

func (a *AAA) Login(name, password string) (string, error) {
    hashed, err := a.rep.GetPasswordHash(context.Background(), name)
    if err != nil {
        return "", errors.New("invalid credentials")
    }
    if err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)); err != nil {
        return "", errors.New("invalid credentials")
    }

    return a.issueToken(name)
}

// Verify checks the token signature and standard claims (exp, nbf, iss)
// and returns the subject (user name) of a valid token.
func (a *AAA) Verify(tokenString string) (string, error) {
    claims := jwt.RegisteredClaims{}
    _, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (any, error) {
        // reject tokens signed with any other algorithm (e.g. "none")
        if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
        }
        return a.secret, nil
    },
        jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
        jwt.WithIssuer(issuerName),
        jwt.WithExpirationRequired(),
        jwt.WithTimeFunc(time.Now),
    )
    if err != nil {
        if errors.Is(err, jwt.ErrTokenExpired) {
            return "", errors.New("token expired")
        }
        return "", fmt.Errorf("invalid token: %w", err)
    }
    if claims.Subject == "" {
        return "", errors.New("token has no subject")
    }
    return claims.Subject, nil
}
