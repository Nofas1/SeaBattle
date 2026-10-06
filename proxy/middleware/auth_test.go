package mw

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type authRepoStub struct {
	passwordHash   string
	getError       error
	registerError  error
	registeredName string
	registeredHash string
}

func (rep *authRepoStub) RegisterUser(_ context.Context, name, passwordHash string) error {
	rep.registeredName = name
	rep.registeredHash = passwordHash
	return rep.registerError
}

func (rep *authRepoStub) GetPasswordHash(context.Context, string) (string, error) {
	return rep.passwordHash, rep.getError
}

func TestNewRequiresStrongJWTSecret(test *testing.T) {
	tests := []struct {
		name    string
		secret  string
		wantErr bool
	}{
		{name: "missing", wantErr: true},
		{name: "too short", secret: "short-secret", wantErr: true},
		{name: "valid", secret: "01234567890123456789012345678901"},
	}
	for _, testCase := range tests {
		test.Run(testCase.name, func(test *testing.T) {
			test.Setenv(jwtSecretEnv, testCase.secret)
			_, err := New(time.Minute, nil, nil)
			if (err != nil) != testCase.wantErr {
				test.Fatalf("New() error = %v; wantErr=%v", err, testCase.wantErr)
			}
		})
	}
}

func TestNewDefaultsNonPositiveTTL(test *testing.T) {
	test.Setenv(jwtSecretEnv, "01234567890123456789012345678901")
	auth, err := New(0, nil, nil)
	if err != nil {
		test.Fatalf("New() error = %v", err)
	}
	if auth.tokenTTL != defaultTokenTTL {
		test.Errorf("token TTL = %v; want %v", auth.tokenTTL, defaultTokenTTL)
	}
}

func TestIssueAndVerifyToken(test *testing.T) {
	auth := AAA{secret: []byte("01234567890123456789012345678901"), tokenTTL: time.Minute}
	token, err := auth.issueToken("captain")
	if err != nil {
		test.Fatalf("issueToken() error = %v", err)
	}
	name, err := auth.Verify(token)
	if err != nil {
		test.Fatalf("Verify() error = %v", err)
	}
	if name != "captain" {
		test.Errorf("Verify() subject = %q; want captain", name)
	}
}

func TestVerifyRejectsInvalidTokens(test *testing.T) {
	auth := AAA{secret: []byte("01234567890123456789012345678901"), tokenTTL: time.Minute}
	if _, err := auth.Verify("not-a-token"); err == nil {
		test.Error("Verify() accepted malformed token")
	}

	expired := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    issuerName,
		Subject:   "captain",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
	})
	expiredToken, err := expired.SignedString(auth.secret)
	if err != nil {
		test.Fatalf("sign expired token: %v", err)
	}
	if _, err := auth.Verify(expiredToken); err == nil {
		test.Error("Verify() accepted expired token")
	}

	noSubject, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    issuerName,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}).SignedString(auth.secret)
	if err != nil {
		test.Fatalf("sign subjectless token: %v", err)
	}
	if _, err := auth.Verify(noSubject); err == nil {
		test.Error("Verify() accepted token without a subject")
	}
}

func TestVerifyRejectsUnexpectedSigningMethod(test *testing.T) {
	auth := AAA{secret: []byte("01234567890123456789012345678901"), tokenTTL: time.Minute}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{
		Issuer:    issuerName,
		Subject:   "captain",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	})
	unsigned, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		test.Fatalf("sign unsigned token: %v", err)
	}
	if _, err := auth.Verify(unsigned); err == nil {
		test.Error("Verify() accepted an unsigned token")
	}
}

func TestRegisterHashesPassword(test *testing.T) {
	rep := &authRepoStub{}
	auth := AAA{rep: rep}
	if err := auth.Register("captain", "correct horse battery staple"); err != nil {
		test.Fatalf("Register() error = %v", err)
	}
	if rep.registeredName != "captain" {
		test.Errorf("registered name = %q; want captain", rep.registeredName)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(rep.registeredHash), []byte("correct horse battery staple")); err != nil {
		test.Errorf("stored password is not a valid hash: %v", err)
	}
}

func TestRegisterReturnsRepositoryErrors(test *testing.T) {
	wantError := errors.New("insert failed")
	auth := AAA{rep: &authRepoStub{registerError: wantError}}
	if err := auth.Register("captain", "password"); !errors.Is(err, wantError) {
		test.Errorf("Register() error = %v; want wrapped %v", err, wantError)
	}
}

func TestLoginValidatesPasswordAndIssuesToken(test *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	if err != nil {
		test.Fatalf("hash password: %v", err)
	}
	auth := AAA{
		rep:      &authRepoStub{passwordHash: string(hash)},
		secret:   []byte("01234567890123456789012345678901"),
		tokenTTL: time.Minute,
	}
	token, err := auth.Login("captain", "secret")
	if err != nil {
		test.Fatalf("Login() error = %v", err)
	}
	if name, err := auth.Verify(token); err != nil || name != "captain" {
		test.Errorf("Login() token subject = %q, error = %v; want captain", name, err)
	}
}

func TestLoginRejectsInvalidCredentialsAndRepositoryErrors(test *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	if err != nil {
		test.Fatalf("hash password: %v", err)
	}
	auth := AAA{rep: &authRepoStub{passwordHash: string(hash)}}
	if _, err := auth.Login("captain", "wrong"); err == nil {
		test.Error("Login() accepted an incorrect password")
	}

	readError := errors.New("database unavailable")
	auth.rep = &authRepoStub{getError: readError}
	if _, err := auth.Login("captain", "secret"); err == nil {
		test.Error("Login() accepted repository failure")
	}
}
