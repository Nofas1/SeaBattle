package mw

import (
	"errors"
	// "fmt"
	"log/slog"
	// "os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const secretKey = "LBv2{zlze*v8(|?f6j[+{@9|(>h/98s#X)DrkWsT{dT" // token sign key
const adminRole = "superuser"             // token subject

// Authentication, Authorization, Accounting
type AAA struct {
	users    map[string]string
	tokenTTL time.Duration
	log      *slog.Logger
}

func New(tokenTTL time.Duration, log *slog.Logger) (AAA, error) {
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
		users:    map[string]string{},
		tokenTTL: tokenTTL,
		log:      log,
	}, nil
}

func (a *AAA) Register(name, password string) error {
	if _, exists := a.users[name]; !exists {
		a.users[name] = password
	} else {
		return errors.New("User already exists")
	}
	return nil
}

func (a *AAA) Login(name, password string) (string, error) {
	if a.users[name] != password {
		return "", errors.New("Authorization error")
	}
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject: name,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(a.tokenTTL)),
	})

	token, err := jwtToken.SignedString([]byte(secretKey))
	if err != nil {
		return "", errors.New("Failed to sign token")
	}

	return token, nil
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
