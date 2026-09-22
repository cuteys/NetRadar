package auth

import (
	"testing"
)

func TestAuthService(t *testing.T) {
	svc, err := NewAuthService("admin", "password123", "secret_test_key_1234567890")
	if err != nil {
		t.Fatalf("NewAuthService failed: %v", err)
	}

	// Test credentials
	if !svc.VerifyPassword("password123") {
		t.Errorf("VerifyPassword failed with correct password")
	}
	if svc.VerifyPassword("wrongpassword") {
		t.Errorf("VerifyPassword succeeded with wrong password")
	}

	// Test Token generation and validation via CheckLogin
	token, err := svc.CheckLogin("127.0.0.1", "admin", "password123")
	if err != nil {
		t.Fatalf("CheckLogin failed: %v", err)
	}
	if token == "" {
		t.Fatalf("CheckLogin returned empty token")
	}

	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if claims.Username != "admin" {
		t.Errorf("claims.Username = %q, want admin", claims.Username)
	}

	// Test UpdateCredentials
	newHash, err := svc.UpdateCredentials("admin", "newpassword456")
	if err != nil {
		t.Fatalf("UpdateCredentials failed: %v", err)
	}
	if len(newHash) == 0 {
		t.Fatalf("UpdateCredentials returned empty hash")
	}

	if !svc.VerifyPassword("newpassword456") {
		t.Errorf("VerifyPassword failed with new password")
	}
	if svc.VerifyPassword("password123") {
		t.Errorf("VerifyPassword succeeded with old password")
	}
}
