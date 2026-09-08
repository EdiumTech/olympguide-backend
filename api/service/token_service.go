package service

import (
	"api/utils/constants"
	"api/utils/errs"
	"errors"
	"github.com/golang-jwt/jwt/v4"
	"os"
	"time"
)

type ITokenService interface {
	GenerateIDToken(userID uint) (string, error)
	ValidateIDToken(tokenString string) (uint, error)
	GenerateEmailToken(email string) (string, error)
	ValidateEmailToken(tokenString string) (string, error)
}

type TokenService struct {
	secretKey string
}

func NewTokenService() *TokenService {
	secretKey := os.Getenv("TOKEN_SECRET")
	if len(secretKey) < 32 {
		panic("TOKEN_SECRET must contain at least 32 bytes")
	}
	return &TokenService{secretKey: secretKey}
}

func (s *TokenService) GenerateIDToken(userID uint) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(constants.IDTokenTTL).Unix(),
	})
	return token.SignedString([]byte(s.secretKey))
}

func (s *TokenService) ValidateIDToken(tokenString string) (uint, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.secretKey), nil
	}, jwt.WithValidMethods([]string{"HS256"}))

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return 0, errs.TokenExpired
		}
		return 0, errs.InvalidToken
	}

	if !token.Valid {
		return 0, errs.InvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, errs.InvalidToken
	}

	userID, ok := claims["user_id"].(float64)
	if !ok {
		return 0, errs.InvalidToken
	}

	return uint(userID), nil
}

func (s *TokenService) GenerateEmailToken(email string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email": email,
		"exp":   time.Now().Add(constants.EmailTokenTTL).Unix(),
	})
	return token.SignedString([]byte(s.secretKey))
}

func (s *TokenService) ValidateEmailToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.secretKey), nil
	}, jwt.WithValidMethods([]string{"HS256"}))

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", errs.TokenExpired
		}
		return "", errs.InvalidToken
	}

	if !token.Valid {
		return "", errs.InvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errs.InvalidToken
	}

	email, ok := claims["email"].(string)
	if !ok {
		return "", errs.InvalidToken
	}

	return email, nil
}
