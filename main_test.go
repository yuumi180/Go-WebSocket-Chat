package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleRegister(t *testing.T) {
	InitDB()

	tests := []struct {
		name         string
		payload      AuthPayload
		expectedCode int
	}{
		{
			name: "valid registration",
			payload: AuthPayload{
				Username: "testuser",
				Password: "testpass",
			},
			expectedCode: http.StatusCreated,
		},
		{
			name: "empty username",
			payload: AuthPayload{
				Username: "",
				Password: "testpass",
			},
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
			rr := httptest.NewRecorder()

			handleRegister(rr, req)

			if rr.Code != tt.expectedCode {
				t.Errorf("expected status %d, got %d", tt.expectedCode, rr.Code)
			}
		})
	}
}

func TestGenerateAndValidateJWT(t *testing.T) {
	username := "testuser"

	token, err := GenerateJWT(username)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	validatedUser, err := ValidateJWT(token)
	if err != nil {
		t.Fatalf("failed to validate JWT: %v", err)
	}

	if validatedUser != username {
		t.Errorf("expected username %s, got %s", username, validatedUser)
	}
}

func TestExpiredJWT(t *testing.T) {
	// 测试过期 token
	oldToken := "expired_token_here"
	_, err := ValidateJWT(oldToken)
	if err == nil {
		t.Error("expected error for expired token, got nil")
	}
}
