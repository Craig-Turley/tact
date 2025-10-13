package auth

import (
	"net/http"
	"os"

	"github.com/Craig-Turley/task-scheduler.git/pkg/utils"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/google"
)

var (
	key            string
	MaxAge         = 86400 * 30
	IsProd         = false
	JWTSecret      string
	CookieName     string
	CookieDomain   string
	CookieSecure   bool
	CookieSameSite = http.SameSiteLaxMode
)

const AUTHORIZATION_HEADER = "Authorization"
const BEARER_PREFIX = "Bearer "

// TODO: find a better way to pass isProd
func NewAuth() {
	key = utils.Getenv("SESSION_KEY", "")
	JWTSecret = os.Getenv("JWT_SECRET")
	CookieName = utils.Getenv("SESSION_COOKIE_NAME", "auth")
	CookieDomain = utils.Getenv("SESSION_COOKIE_DOMAIN", "")
	CookieSecure = utils.Getenv("SESSION_COOKIE_SECURE", "false") == "true"

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		panic("GOOGLE_CLIENT_ID and/or GOOGLE_CLIENT_SECRET not loaded correctly")
	}
	if key == "" {
		panic("SESSION_KEY not set")
	}

	store := sessions.NewCookieStore([]byte(key))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	}

	gothic.Store = store
	goth.UseProviders(
		google.New(clientID, clientSecret, "http://localhost:8080/api/v1/auth/google/callback", "email", "profile"),
	)
}

func KeyFunc(token *jwt.Token) (any, error) {
	if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, utils.NewError("unexpected signing method: %v", token.Header["alg"])
	}

	return JWTSecret, nil
}

var JWTParser = jwt.NewParser(
	jwt.WithValidMethods([]string{"HS256"}),
)

func VerifyToken(tokenStr string) (*jwt.Token, error) {
	return JWTParser.Parse(tokenStr, KeyFunc)
}
