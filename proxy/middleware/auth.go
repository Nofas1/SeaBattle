package mw

import (
	"errors"
	"sea_battle/proxy/repository"

	"fmt"
	"log/slog"
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const secretKey = "LBv2{zlze*v8(|?f6j[+{@9|(>h/98s#X)DrkWsT{dT" // token sign key
const adminRole = "superuser"             // token subject

// Authentication, Authorization, Accounting
type AAA struct {
	rep *repository.Repo
	users    map[string]string
	tokenTTL time.Duration
	log      *slog.Logger
}

func New(tokenTTL time.Duration, log *slog.Logger, rep *repository.Repo) (AAA, error) {
	// const adminUser = "ADMIN_USER"
	// const adminPass = "ADMIN_PASSWORD"
	// user, ok := os.LookupEnv(adminUser)
	// if !ok {
	// 	return AAA{}, fmt.Errorf("could not get admin user from enviroment")
	// }
	// password, ok := os.LookupEnv(adminPass)
	// if !ok {
	// 	return AAA{}, fmt.Errorf("could not get admin password from enviroment")
	// }

	return AAA{
		rep: rep,
		users:    map[string]string{},
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
	// 	a.users[name] = password
	// } else {
	// 	return errors.New("User already exists")
	// }
	return nil
}

func (a *AAA) Login(name, password string) (string, error) {
	hashed, err := a.rep.GetPasswordHash(context.Background(), name)
    if err != nil {
        return "", errors.New("invalid credentials")
    }
    if err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)); err != nil {
        return "", errors.New("invalid credentials")
    }

    jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
        Subject:   name,
        ExpiresAt: jwt.NewNumericDate(time.Now().Add(a.tokenTTL)),
    })
    token, err := jwtToken.SignedString([]byte(secretKey))
    if err != nil {
        return "", fmt.Errorf("failed to sign token: %w", err)
    }
    return token, nil
	// if a.users[name] != password {
	// 	return "", errors.New("Authorization error")
	// }
	// jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
	// 	Subject: name,
	// 	ExpiresAt: jwt.NewNumericDate(time.Now().Add(a.tokenTTL)),
	// })

	// token, err := jwtToken.SignedString([]byte(secretKey))
	// if err != nil {
	// 	return "", errors.New("Failed to sign token")
	// }

	// return token, nil
}

func (a *AAA) Verify(tokenString string) (string, error) {
	claims := jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (any, error) {
		return []byte(secretKey), nil
	})
	if err != nil {
		return "", errors.New("Failed to parse token")
	}

	if !token.Valid {
		return "", errors.New("Token expired")
	}
	return claims.Subject, nil
}
