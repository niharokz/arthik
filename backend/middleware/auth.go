package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"arthik/config"
	"arthik/utils"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	jwtSecret     []byte
	loginAttempts = make(map[string]*loginAttempt)
	loginMutex    sync.RWMutex
)

type loginAttempt struct {
	attempts    int
	lastAttempt time.Time
}

// Claims represents JWT claims
type Claims struct {
	jwt.RegisteredClaims
}

// SetJWTSecret sets the JWT secret key
func SetJWTSecret(secret []byte) {
	jwtSecret = secret
}

// Auth middleware validates JWT tokens
func Auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			utils.LogAudit(r.RemoteAddr, "unknown", "ACCESS_DENIED", r.URL.Path, false)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			utils.LogAudit(r.RemoteAddr, "unknown", "INVALID_TOKEN_FORMAT", r.URL.Path, false)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			utils.LogAudit(r.RemoteAddr, "unknown", "INVALID_TOKEN", r.URL.Path, false)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

// GenerateToken creates a new JWT token
func GenerateToken() (string, error) {
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(config.TokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword verifies a password against a hash
func VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// CheckLoginAttempts checks if login is allowed based on failed attempts
func CheckLoginAttempts(ip string) bool {
	loginMutex.Lock()
	defer loginMutex.Unlock()

	attempt, exists := loginAttempts[ip]
	if !exists {
		loginAttempts[ip] = &loginAttempt{attempts: 1, lastAttempt: time.Now()}
		return true
	}

	// Reset after window expires
	if time.Since(attempt.lastAttempt) > config.LoginAttemptWindow {
		attempt.attempts = 1
		attempt.lastAttempt = time.Now()
		return true
	}

	// Block after max attempts
	if attempt.attempts >= config.MaxLoginAttempts {
		return false
	}

	attempt.attempts++
	attempt.lastAttempt = time.Now()
	return true
}

// ResetLoginAttempts resets login attempts for an IP
func ResetLoginAttempts(ip string) {
	loginMutex.Lock()
	defer loginMutex.Unlock()
	delete(loginAttempts, ip)
}