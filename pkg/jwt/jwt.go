package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenType Token 类型标识
type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

// Claims 自定义 JWT Claims
type Claims struct {
	UserID    uint64    `json:"user_id"`
	Username  string    `json:"username"`
	TokenType TokenType `json:"token_type"` // 区分 access 和 refresh
	jwt.RegisteredClaims
}

// TokenPair 双 Token 对
type TokenPair struct {
	AccessToken           string `json:"access_token"`
	RefreshToken          string `json:"refresh_token"`
	AccessTokenExpiresAt  int64  `json:"access_token_expires_at"`  // Unix 时间戳
	RefreshTokenExpiresAt int64  `json:"refresh_token_expires_at"` // Unix 时间戳
}

// JWTManager JWT 管理器
type JWTManager struct {
	secret          []byte
	accessTokenExp  time.Duration
	refreshTokenExp time.Duration
	issuer          string
}

// NewJWTManager 创建 JWT 管理器
// secret: 签名密钥
// accessTokenExp: Access Token 过期时间
// refreshTokenExp: Refresh Token 过期时间
// issuer: 签发者
func NewJWTManager(secret string, accessTokenExp, refreshTokenExp time.Duration, issuer string) (*JWTManager, error) {
	if secret == "" {
		return nil, fmt.Errorf("jwt secret cannot be empty")
	}
	if accessTokenExp <= 0 || refreshTokenExp <= 0 {
		return nil, fmt.Errorf("token expiration must be positive")
	}
	if accessTokenExp >= refreshTokenExp {
		return nil, fmt.Errorf("access token expiration must be less than refresh token expiration")
	}

	return &JWTManager{
		secret:          []byte(secret),
		accessTokenExp:  accessTokenExp,
		refreshTokenExp: refreshTokenExp,
		issuer:          issuer,
	}, nil
}

// GenerateTokenPair 生成双 Token 对
func (m *JWTManager) GenerateTokenPair(userID uint64, username string) (*TokenPair, error) {
	now := time.Now()

	// 生成 Access Token
	accessClaims := &Claims{
		UserID:    userID,
		Username:  username,
		TokenType: AccessToken,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTokenExp)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    m.issuer,
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenStr, err := accessToken.SignedString(m.secret)
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// 生成 Refresh Token
	refreshClaims := &Claims{
		UserID:    userID,
		Username:  username,
		TokenType: RefreshToken,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshTokenExp)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    m.issuer,
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenStr, err := refreshToken.SignedString(m.secret)
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:           accessTokenStr,
		RefreshToken:          refreshTokenStr,
		AccessTokenExpiresAt:  now.Add(m.accessTokenExp).Unix(),
		RefreshTokenExpiresAt: now.Add(m.refreshTokenExp).Unix(),
	}, nil
}

// ParseToken 解析并验证 Token
func (m *JWTManager) ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名算法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// ParseAccessToken 解析并验证 Access Token
func (m *JWTManager) ParseAccessToken(tokenStr string) (*Claims, error) {
	claims, err := m.ParseToken(tokenStr)
	if err != nil {
		return nil, err
	}

	if claims.TokenType != AccessToken {
		return nil, fmt.Errorf("expected access token, got %s", claims.TokenType)
	}

	return claims, nil
}

// ParseRefreshToken 解析并验证 Refresh Token
func (m *JWTManager) ParseRefreshToken(tokenStr string) (*Claims, error) {
	claims, err := m.ParseToken(tokenStr)
	if err != nil {
		return nil, err
	}

	if claims.TokenType != RefreshToken {
		return nil, fmt.Errorf("expected refresh token, got %s", claims.TokenType)
	}

	return claims, nil
}

// RefreshAccessToken 使用 Refresh Token 刷新，获取新的 Token 对
func (m *JWTManager) RefreshAccessToken(refreshTokenStr string) (*TokenPair, error) {
	// 验证 Refresh Token
	claims, err := m.ParseRefreshToken(refreshTokenStr)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	// 生成新的 Token 对
	return m.GenerateTokenPair(claims.UserID, claims.Username)
}
