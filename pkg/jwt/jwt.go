package jwt

import (
	"YoudaoNoteLm/pkg/config"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrTokenInvalid     = errors.New("token 无效")
	ErrTokenExpired     = errors.New("token 已过期")
	ErrTokenTypeInvalid = errors.New("token 类型错误")
)

// TokenPair 双 token 结构
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// GenerateAccessToken 生成 Access Token（15 分钟）
func GenerateAccessToken(userID uint, username string) (string, error) {
	cfg := config.Get().JWT
	exp := cfg.GetAccessTokenExp()

	claims := CustomClaims{
		UserID:    userID,
		Username:  username,
		TokenType: AccessToken,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(exp)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    cfg.GetIssuer(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Secret))
}

// GenerateRefreshToken 生成 Refresh Token（7 天）
func GenerateRefreshToken(userID uint, username string) (string, error) {
	cfg := config.Get().JWT
	exp := cfg.GetRefreshTokenExp()

	claims := CustomClaims{
		UserID:    userID,
		Username:  username,
		TokenType: RefreshToken,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(exp)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    cfg.GetIssuer(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Secret))
}

// GenerateTokenPair 生成 Access + Refresh Token 对
func GenerateTokenPair(userID uint, username string) (*TokenPair, error) {
	accessToken, err := GenerateAccessToken(userID, username)
	if err != nil {
		return nil, err
	}

	refreshToken, err := GenerateRefreshToken(userID, username)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// ParseToken 解析 JWT Token
func ParseToken(tokenString string) (*CustomClaims, error) {
	cfg := config.Get().JWT

	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.Secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}

	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrTokenInvalid
}

// RefreshAccessToken 用 Refresh Token 换取新的 Access Token
func RefreshAccessToken(refreshTokenString string) (*TokenPair, error) {
	claims, err := ParseToken(refreshTokenString)
	if err != nil {
		return nil, err
	}

	// 必须是 refresh token
	if claims.TokenType != RefreshToken {
		return nil, ErrTokenTypeInvalid
	}

	return GenerateTokenPair(claims.UserID, claims.Username)
}
