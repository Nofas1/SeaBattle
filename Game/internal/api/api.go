package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func AuthRequestHandler(url, username, password string) (string, error) {
	body, err := json.Marshal(map[string]string{
		"name":     username,
		"password": password,
	})
	if err != nil {
		return "", err
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("%s", strings.TrimSpace(string(respBody)))
	}

	var authResp AuthResponse
	if err := json.Unmarshal(respBody, &authResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}
	if authResp.Token == "" {
		return "", fmt.Errorf("empty token in response")
	}

	// var authResp AuthResponse
	// if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
	// 	return "", err
	// }
	return authResp.Token, nil
}
