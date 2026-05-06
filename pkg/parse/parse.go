package jwt

import (
	"errors"
	"fmt"
	"os"

	"github.com/golang-jwt/jwt/v4"
)

func ParseToken(tokenString string) (map[string]interface{}, error) {
	// .env dan secretni olamiz
	secretKey := os.Getenv("JWT_SECRET_KEY")
	if secretKey == "" {
		return nil, errors.New("JWT_SECRET_KEY is not set in environment")
	}

	// Tokenni parsing qilish
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// HMAC usulda imzolanganligini tekshiramiz
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secretKey), nil
	})
	if err != nil {
		return nil, err
	}

	// Token valid bo‘lsa, claimlarni olish
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token or claims")
}

func ParsePageOrder(order []int) ([]int, error) {
	for _, page := range order {
		if page <= 0 {
			return nil, fmt.Errorf("invalid page number: %d", page)
		}
	}
	return order, nil
}
