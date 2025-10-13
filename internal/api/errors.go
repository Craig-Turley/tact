package api

import (
	"log"
	"net/http"
	"net/url"

	"github.com/Craig-Turley/task-scheduler.git/pkg/utils"
)

const CONSOLE_LOG = true // this is for dev convienience might remove

func (s *Server) internalServerError(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		err = utils.NewError("Internal Server Error")
	}

	if CONSOLE_LOG {
		log.Println(err.Error())
	}

	// s.logger.Errorw("internal error", "method", r.Method, "path", r.URL.Path, "error", err.Error())

	writeJSONError(w, http.StatusInternalServerError, "the server encountered a problem")
}

// this method is specifcally for the case of auth failing
func (s *Server) clientErrorRedirect(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		err = utils.NewError("Internal Server Error")
	}

	if CONSOLE_LOG {
		log.Println(err.Error())
	}

	// send the error back in a cookie so the front end can read it
	// s.logger.Errorw("internal error", "method", r.Method, "path", r.URL.Path, "error", err.Error())
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_error",
		Value:    url.QueryEscape("Login failed. Please try again."),
		Path:     "/",
		MaxAge:   60, // 1 minute may or may not change
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
	})
	http.Redirect(w, r, s.Origin+"auth/callback?error=1", http.StatusSeeOther)
}

func (s *Server) forbiddenResponse(w http.ResponseWriter, r *http.Request) {
	// s.logger.Warnw("forbidden", "method", r.Method, "path", r.URL.Path, "error")

	writeJSONError(w, http.StatusForbidden, "forbidden")
}

func (s *Server) badRequestResponse(w http.ResponseWriter, r *http.Request, err error) {
	if CONSOLE_LOG && err != nil {
		log.Println(err.Error())
	}

	// s.logger.Warnf("bad request", "method", r.Method, "path", r.URL.Path, "error", err.Error())

	writeJSONError(w, http.StatusBadRequest, err.Error())
}

func (s *Server) multiStatusReponse(w http.ResponseWriter, r *http.Request, err error) {
	if CONSOLE_LOG && err != nil {
		log.Println(err.Error())
	}

	writeJSONError(w, http.StatusMultiStatus, err.Error())
}

func (s *Server) conflictResponse(w http.ResponseWriter, r *http.Request, err error) {
	if CONSOLE_LOG && err != nil {
		log.Println(err.Error())
	}

	// s.logger.Errorf("conflict response", "method", r.Method, "path", r.URL.Path, "error", err.Error())

	writeJSONError(w, http.StatusConflict, err.Error())
}

func (s *Server) notFoundResponse(w http.ResponseWriter, r *http.Request, err error) {
	if CONSOLE_LOG && err != nil {
		log.Println(err.Error())
	}

	// s.logger.Warnf("not found error", "method", r.Method, "path", r.URL.Path, "error", err.Error())

	writeJSONError(w, http.StatusNotFound, "not found")
}

func (s *Server) unauthorizedErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	if CONSOLE_LOG && err != nil {
		log.Println(err.Error())
	}

	// s.logger.Warnf("unauthorized error", "method", r.Method, "path", r.URL.Path, "error", err.Error())

	writeJSONError(w, http.StatusUnauthorized, "unauthorized")
}

func UnauthorizedErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	if CONSOLE_LOG && err != nil {
		log.Println(err.Error())
	}

	// s.logger.Warnf("unauthorized error", "method", r.Method, "path", r.URL.Path, "error", err.Error())

	writeJSONError(w, http.StatusUnauthorized, "unauthorized")
}

// func (s *Server) unauthorizedBasicErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
// 	if CONSOLE_LOG && err != nil {
// 		log.Println(err.Error())
// 	}
//
// 	// s.logger.Warnf("unauthorized basic error", "method", r.Method, "path", r.URL.Path, "error", err.Error())
//
// 	w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
// 	log.Println(err)
//
// 	writeJSONError(w, http.StatusUnauthorized, "unauthorized")
// }

func (s *Server) rateLimitExceededResponse(w http.ResponseWriter, r *http.Request, retryAfter string) {
	// s.logger.Warnw("rate limit exceeded", "method", r.Method, "path", r.URL.Path)

	w.Header().Set("Retry-After", retryAfter)

	writeJSONError(w, http.StatusTooManyRequests, "rate limit exceeded, retry after: "+retryAfter)
}
