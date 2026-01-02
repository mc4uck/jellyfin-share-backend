package middleware

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	ShareSessionCookieName = "jfshare_session"
	cookieMaxAge           = 24 * time.Hour
)

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(bytes), nil
}

// CheckPassword verifies a password against a bcrypt hash
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateToken generates a cryptographically secure random token
func GenerateToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

// GenerateSecureToken generates a URL-safe token of specified length
func GenerateSecureToken(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:length]
}

// ShareSessionManager handles password-protected share sessions
type ShareSessionManager struct {
	secretKey []byte
}

func NewShareSessionManager(secretKey string) *ShareSessionManager {
	return &ShareSessionManager{
		secretKey: []byte(secretKey),
	}
}

// CreateSessionToken creates a signed session token for a share
func (m *ShareSessionManager) CreateSessionToken(shareToken string) string {
	expiry := time.Now().Add(cookieMaxAge).Unix()
	data := fmt.Sprintf("%s:%d", shareToken, expiry)

	mac := hmac.New(sha256.New, m.secretKey)
	mac.Write([]byte(data))
	signature := hex.EncodeToString(mac.Sum(nil))

	return base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", data, signature)))
}

// ValidateSessionToken validates a session token and returns the share token if valid
func (m *ShareSessionManager) ValidateSessionToken(token string) (string, bool) {
	decoded, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return "", false
	}

	parts := strings.Split(string(decoded), ":")
	if len(parts) != 3 {
		return "", false
	}

	shareToken := parts[0]
	expiryStr := parts[1]
	providedSig := parts[2]

	// Verify signature
	data := fmt.Sprintf("%s:%s", shareToken, expiryStr)
	mac := hmac.New(sha256.New, m.secretKey)
	mac.Write([]byte(data))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(providedSig), []byte(expectedSig)) {
		return "", false
	}

	// Check expiry
	var expiry int64
	fmt.Sscanf(expiryStr, "%d", &expiry)
	if time.Now().Unix() > expiry {
		return "", false
	}

	return shareToken, true
}

// SetSessionCookie sets the session cookie for a password-protected share
func (m *ShareSessionManager) SetSessionCookie(w http.ResponseWriter, shareToken string) {
	token := m.CreateSessionToken(shareToken)
	http.SetCookie(w, &http.Cookie{
		Name:     ShareSessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(cookieMaxAge.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// GetSessionFromCookie extracts and validates the session from the cookie
func (m *ShareSessionManager) GetSessionFromCookie(r *http.Request, shareToken string) bool {
	cookie, err := r.Cookie(ShareSessionCookieName)
	if err != nil {
		return false
	}

	tokenShareToken, valid := m.ValidateSessionToken(cookie.Value)
	if !valid {
		return false
	}

	return tokenShareToken == shareToken
}
