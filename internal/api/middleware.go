package api

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Craig-Turley/task-scheduler.git/internal/auth"
	"github.com/Craig-Turley/task-scheduler.git/pkg/utils"
)

type Middleware func(h http.Handler) http.Handler

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Println(r.Method, r.URL.Path, time.Since(start))
	})
}

func Authorization(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenHeader := r.Header.Get(auth.AUTHORIZATION_HEADER)
		if len(tokenHeader) == 0 {
			UnauthorizedErrorResponse(w, r, utils.NewError("Bearer token not provided"))
			return
		}

		tokenStr := strings.Trim(tokenHeader, auth.BEARER_PREFIX)
		if len(tokenStr) == 0 {
			UnauthorizedErrorResponse(w, r, utils.NewError("Bearer token not provided"))
			return
		}

		if _, err := auth.VerifyToken(tokenStr); err != nil {
			UnauthorizedErrorResponse(w, r, utils.NewError("Invalid token"))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func MiddlewareChain(xs ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		for i := len(xs) - 1; i >= 0; i-- {
			x := xs[i]
			next = x(next)
		}

		return next
	}
}
