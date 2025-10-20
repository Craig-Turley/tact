package api

import (
	"log"
	"net/http"
	"time"

	"github.com/Craig-Turley/task-scheduler.git/internal/auth"
	"github.com/Craig-Turley/task-scheduler.git/pkg/utils"
)

type Middleware func(h http.Handler) http.Handler

// type for the Logging middleware to get status for logging purposes
type WrappedWriter struct {
	http.ResponseWriter
	Status int
}

func NewWrappedWriter(w http.ResponseWriter) *WrappedWriter {
	return &WrappedWriter{
		ResponseWriter: w,
	}
}

func (w *WrappedWriter) WriteHeader(statusCode int) {
	w.Status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrappedWriter := NewWrappedWriter(w)
		next.ServeHTTP(wrappedWriter, r)
		log.Println(r.Method, r.URL.Path, wrappedWriter.Status, time.Since(start))
	})
}

func Authorization(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenCookie, err := r.Cookie(auth.CookieName)
		if err != nil {
			UnauthorizedErrorResponse(w, r, utils.NewError("Invalid Auth Cookie"))
			return
		}

		if _, err := auth.VerifyToken(tokenCookie.Value); err != nil {
			UnauthorizedErrorResponse(w, r, err)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func EnableCors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
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
