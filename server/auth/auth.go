package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const UserContextKey = contextKey("auth_user")

type CustomClaims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type AuthService struct {
	jwtSecret      []byte
	adminUsername  string
	adminPwdHash   []byte
	rateLimitMu    sync.Mutex
	failedAttempts map[string]*attemptInfo
}

type attemptInfo struct {
	count       int
	blockedTill time.Time
}

func NewAuthService(adminUser, adminPassword, jwtSecret string) (*AuthService, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	s := &AuthService{
		jwtSecret:      []byte(jwtSecret),
		adminUsername:  adminUser,
		adminPwdHash:   hash,
		failedAttempts: make(map[string]*attemptInfo),
	}

	// 定期清理已过封禁期的 IP 记录，防止内存泄漏
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for now := range ticker.C {
			s.rateLimitMu.Lock()
			for ip, info := range s.failedAttempts {
				if !info.blockedTill.IsZero() && now.After(info.blockedTill) {
					delete(s.failedAttempts, ip)
				}
			}
			s.rateLimitMu.Unlock()
		}
	}()

	return s, nil
}

func (s *AuthService) GetUsername() string {
	return s.adminUsername
}

func (s *AuthService) SetPasswordHash(username string, hash []byte) {
	s.adminUsername = username
	s.adminPwdHash = hash
}

func (s *AuthService) VerifyPassword(password string) bool {
	return bcrypt.CompareHashAndPassword(s.adminPwdHash, []byte(password)) == nil
}

func (s *AuthService) UpdateCredentials(newUsername, newPassword string) ([]byte, error) {
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	s.adminUsername = newUsername
	s.adminPwdHash = newHash
	return newHash, nil
}

func (s *AuthService) CheckLogin(ip, username, password string) (string, error) {
	s.rateLimitMu.Lock()
	now := time.Now()
	info, exists := s.failedAttempts[ip]
	if exists && now.Before(info.blockedTill) {
		s.rateLimitMu.Unlock()
		return "", errors.New("登录失败次数过多，请稍后再试")
	}
	s.rateLimitMu.Unlock()

	if username != s.adminUsername {
		s.recordFailedAttempt(ip)
		return "", errors.New("用户名或密码错误")
	}

	err := bcrypt.CompareHashAndPassword(s.adminPwdHash, []byte(password))
	if err != nil {
		s.recordFailedAttempt(ip)
		return "", errors.New("用户名或密码错误")
	}

	s.rateLimitMu.Lock()
	delete(s.failedAttempts, ip)
	s.rateLimitMu.Unlock()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, CustomClaims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   username,
		},
	})

	return token.SignedString(s.jwtSecret)
}

func (s *AuthService) recordFailedAttempt(ip string) {
	s.rateLimitMu.Lock()
	defer s.rateLimitMu.Unlock()

	info, exists := s.failedAttempts[ip]
	if !exists {
		info = &attemptInfo{}
		s.failedAttempts[ip] = info
	}

	info.count++
	if info.count >= 5 {
		info.blockedTill = time.Now().Add(5 * time.Minute)
		info.count = 0
	}
}

func (s *AuthService) ValidateToken(tokenStr string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &CustomClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("签名方式异常")
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("无效的凭据")
}

func (s *AuthService) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := s.extractToken(r)
		if tokenStr == "" {
			http.Error(w, `{"error":"unauthorized","message":"missing authorization"}`, http.StatusUnauthorized)
			return
		}

		claims, err := s.ValidateToken(tokenStr)
		if err != nil {
			http.Error(w, `{"error":"unauthorized","message":"session expired"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, claims.Username)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *AuthService) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return s.HTTPMiddleware(next).ServeHTTP
}

func (s *AuthService) ValidateRequest(r *http.Request) bool {
	tokenStr := s.extractToken(r)
	if tokenStr == "" {
		return false
	}
	_, err := s.ValidateToken(tokenStr)
	return err == nil
}

func (s *AuthService) extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}

	if cookie, err := r.Cookie("netradar_session"); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	if qToken := r.URL.Query().Get("token"); qToken != "" {
		return qToken
	}

	return ""
}
