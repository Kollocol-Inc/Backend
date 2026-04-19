package jwt

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

const testSecret = "test-secret-key-for-jwt-testing"

func TestGenerateTokenPair(t *testing.T) {
	userID := "user-123"
	email := "test@example.com"

	pair, err := GenerateTokenPair(userID, email, "user", testSecret)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	if pair.AccessToken == "" {
		t.Error("AccessToken is empty")
	}
	if pair.RefreshToken == "" {
		t.Error("RefreshToken is empty")
	}
	if pair.AccessToken == pair.RefreshToken {
		t.Error("AccessToken and RefreshToken should differ")
	}
}

func TestValidateAccessToken_Valid(t *testing.T) {
	pair, err := GenerateTokenPair("user-123", "test@example.com", "user", testSecret)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	claims, err := ValidateAccessToken(pair.AccessToken, testSecret)
	if err != nil {
		t.Fatalf("ValidateAccessToken() error = %v", err)
	}

	if claims.UserID != "user-123" {
		t.Errorf("UserID = %q, want %q", claims.UserID, "user-123")
	}
	if claims.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", claims.Email, "test@example.com")
	}
	if claims.JTI == "" {
		t.Error("JTI should not be empty for access tokens")
	}
}

func TestValidateAccessToken_WrongSecret(t *testing.T) {
	pair, _ := GenerateTokenPair("user-123", "test@example.com", "user", testSecret)
	_, err := ValidateAccessToken(pair.AccessToken, "wrong-secret")
	if err == nil {
		t.Error("ValidateAccessToken() should fail with wrong secret")
	}
}

func TestValidateAccessToken_Expired(t *testing.T) {
	claims := &Claims{
		UserID: "user-123",
		Email:  "test@example.com",
		JTI:    "test-jti",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(testSecret))

	_, err := ValidateAccessToken(tokenString, testSecret)
	if err == nil {
		t.Error("ValidateAccessToken() should fail for expired token")
	}
}

func TestValidateAccessToken_InvalidString(t *testing.T) {
	_, err := ValidateAccessToken("not-a-jwt-token", testSecret)
	if err == nil {
		t.Error("ValidateAccessToken() should fail for invalid token string")
	}
}

func TestValidateRefreshToken_Valid(t *testing.T) {
	pair, _ := GenerateTokenPair("user-456", "refresh@example.com", "user", testSecret)

	claims, err := ValidateRefreshToken(pair.RefreshToken, testSecret)
	if err != nil {
		t.Fatalf("ValidateRefreshToken() error = %v", err)
	}

	if claims.UserID != "user-456" {
		t.Errorf("UserID = %q, want %q", claims.UserID, "user-456")
	}
	if claims.Email != "refresh@example.com" {
		t.Errorf("Email = %q, want %q", claims.Email, "refresh@example.com")
	}
}

func TestValidateRefreshToken_WrongSecret(t *testing.T) {
	pair, _ := GenerateTokenPair("user-456", "test@example.com", "user", testSecret)
	_, err := ValidateRefreshToken(pair.RefreshToken, "wrong-secret")
	if err == nil {
		t.Error("ValidateRefreshToken() should fail with wrong secret")
	}
}

func TestExtractJTI(t *testing.T) {
	pair, _ := GenerateTokenPair("user-789", "jti@example.com", "user", testSecret)

	jti, err := ExtractJTI(pair.AccessToken)
	if err != nil {
		t.Fatalf("ExtractJTI() error = %v", err)
	}
	if jti == "" {
		t.Error("JTI should not be empty")
	}
}

func TestExtractJTI_InvalidToken(t *testing.T) {
	_, err := ExtractJTI("garbage")
	if err == nil {
		t.Error("ExtractJTI() should fail for invalid token")
	}
}

func TestExtractJTI_RefreshTokenHasEmptyJTI(t *testing.T) {
	pair, _ := GenerateTokenPair("user-789", "jti@example.com", "user", testSecret)

	jti, err := ExtractJTI(pair.RefreshToken)
	if err != nil {
		t.Fatalf("ExtractJTI() error = %v", err)
	}
	if jti != "" {
		t.Errorf("Refresh token JTI should be empty, got %q", jti)
	}
}
