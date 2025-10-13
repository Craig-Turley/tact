package user

import (
	"github.com/golang-jwt/jwt/v5"
)

type User struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Name      string `json:"name,omitempty"`
	NickName  string `json:"nick_name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
	Provider  string `json:"provider,omitempty"`
}

func UserFromClaims(claims jwt.MapClaims) User {
	return User{
		UserID:    getStringClaim(claims, "sub"),
		Email:     getStringClaim(claims, "email"),
		Name:      getStringClaim(claims, "name"),
		NickName:  getStringClaim(claims, "nick"),
		AvatarURL: getStringClaim(claims, "avatar"),
		Provider:  getStringClaim(claims, "provider"),
	}
}

func getStringClaim(claims jwt.MapClaims, key string) string {
	if val, ok := claims[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}
