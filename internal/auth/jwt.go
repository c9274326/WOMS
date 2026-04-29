package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/c9274326/woms/internal/domain"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("expired token")
)

type Claims struct {
	Subject string      `json:"sub"`
	Role    domain.Role `json:"role"`
	LineID  string      `json:"line_id,omitempty"`
	Expires int64       `json:"exp"`
}

func CreateToken(secret string, claims Claims, ttl time.Duration) (string, error) {
	if secret == "" {
		return "", errors.New("jwt secret is required")
	}
	claims.Expires = time.Now().Add(ttl).Unix()

	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	encodedHeader := base64.RawURLEncoding.EncodeToString(headerJSON)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := encodedHeader + "." + encodedClaims
	signature := sign(secret, signingInput)
	return signingInput + "." + signature, nil
}

func VerifyToken(secret, token string) (Claims, error) {
	var claims Claims
	if secret == "" || token == "" {
		return claims, ErrInvalidToken
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims, ErrInvalidToken
	}

	signingInput := parts[0] + "." + parts[1]
	expected := sign(secret, signingInput)
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return claims, ErrInvalidToken
	}

	body, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims, ErrInvalidToken
	}
	if err := json.Unmarshal(body, &claims); err != nil {
		return claims, ErrInvalidToken
	}
	if claims.Subject == "" || claims.Role == "" {
		return claims, ErrInvalidToken
	}
	if time.Now().Unix() >= claims.Expires {
		return claims, ErrExpiredToken
	}
	return claims, nil
}

func BearerToken(header string) (string, error) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", fmt.Errorf("%w: missing bearer prefix", ErrInvalidToken)
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if token == "" {
		return "", ErrInvalidToken
	}
	return token, nil
}

func sign(secret, input string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(input))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
