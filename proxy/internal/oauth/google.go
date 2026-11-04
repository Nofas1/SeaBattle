package oauth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/oauth2"
)

const googleUserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"

type Flow struct {
	Config         *oauth2.Config
	FrontendURL    string
	FrontendOrigin string
	UserInfoURL    string
	IsProd         bool
}

func LoadConfigFromEnv() (*Flow, error) {
	redirectURL := strings.TrimSpace(os.Getenv("GOOGLE_REDIRECT_URL"))
	if err := validateHTTPURL("GOOGLE_REDIRECT_URL", redirectURL); err != nil {
		return nil, err
	}
	frontendURL := strings.TrimRight(strings.TrimSpace(os.Getenv("FRONTEND_URL")), "/")
	if err := validateHTTPURL("FRONTEND_URL", frontendURL); err != nil {
		return nil, err
	}
	parsedFrontendURL, err := url.ParseRequestURI(frontendURL)
	if err != nil {
		return nil, fmt.Errorf("parse FRONTEND_URL: %w", err)
	}

	clientID := strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET"))
	credentialsPath := strings.TrimSpace(os.Getenv("GOOGLE_CREDENTIALS_JSON"))
	if credentialsPath != "" {
		if !filepath.IsAbs(credentialsPath) {
			return nil, fmt.Errorf("GOOGLE_CREDENTIALS_JSON must be an absolute file path")
		}
		credentials, err := os.ReadFile(credentialsPath)
		if err != nil {
			return nil, fmt.Errorf("read GOOGLE_CREDENTIALS_JSON: %w", err)
		}
		var document map[string]struct {
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
		}
		if err := json.Unmarshal(credentials, &document); err != nil {
			return nil, fmt.Errorf("decode GOOGLE_CREDENTIALS_JSON: %w", err)
		}
		for _, key := range []string{"web", "installed"} {
			if entry, ok := document[key]; ok {
				clientID = strings.TrimSpace(entry.ClientID)
				clientSecret = strings.TrimSpace(entry.ClientSecret)
				break
			}
		}
	}
	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET or provide GOOGLE_CREDENTIALS_JSON")
	}

	isProd := true
	if value, exists := os.LookupEnv("IS_PROD"); exists {
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("IS_PROD must be a boolean")
		}
		isProd = parsed
	}

	return &Flow{
		Config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
				TokenURL: "https://oauth2.googleapis.com/token",
			},
		},
		FrontendURL:    frontendURL,
		FrontendOrigin: parsedFrontendURL.Scheme + "://" + parsedFrontendURL.Host,
		UserInfoURL:    googleUserInfoURL,
		IsProd:         isProd,
	}, nil
}

func (flow *Flow) SecureCookie() bool {
	if flow.IsProd {
		return true
	}
	redirectURL, err := url.Parse(flow.Config.RedirectURL)
	if err != nil || redirectURL.Scheme != "http" {
		return true
	}
	host := strings.ToLower(redirectURL.Hostname())
	if host == "127.0.0.1" {
		return false
	}
	ip := net.ParseIP(host)
	return ip == nil || !ip.IsLoopback()
}

func NewState() (string, error) {
	state := make([]byte, 32)
	if _, err := rand.Read(state); err != nil {
		return "", fmt.Errorf("generate OAuth state: %w", err)
	}
	return base64.URLEncoding.EncodeToString(state), nil
}

func validateHTTPURL(name, value string) error {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("%s must be an absolute http or https URL", name)
	}
	return nil
}
