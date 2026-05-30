package auth

import (
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID   string `json:"sub"`
	Email    string `json:"email"`
	JFUserID string `json:"jf_user"`
	jwt.RegisteredClaims
}

func (s *Service) issueJWT(userID, email, jfUserID string) (string, error) {
	now := s.clock()
	claims := Claims{
		UserID:   userID,
		Email:    email,
		JFUserID: jfUserID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
			ID:        uuid.NewString(),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(s.signKey)
}

// IssueAccessTokenForTest is a test seam; production code calls issueJWT internally.
func (s *Service) IssueAccessTokenForTest(userID, email, jfUserID string) (string, error) {
	return s.issueJWT(userID, email, jfUserID)
}

func (s *Service) VerifyJWT(token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.signKey, nil
	}, jwt.WithTimeFunc(s.clock))
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid claims")
	}
	return claims, nil
}
