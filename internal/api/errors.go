package api

import (
	"net/http"

	"github.com/Craig-Turley/task-scheduler.git/pkg/utils"
)

func (s *Server) internalServerError(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		err = utils.NewError("Internal Server Error")
	}

	// s.logger.Errorw("internal error", "method", r.Method, "path", r.URL.Path, "error", err.Error())

	writeJSONError(w, http.StatusInternalServerError, "the server encountered a problem")
}

func (s *Server) forbiddenResponse(w http.ResponseWriter, r *http.Request) {
	// s.logger.Warnw("forbidden", "method", r.Method, "path", r.URL.Path, "error")

	writeJSONError(w, http.StatusForbidden, "forbidden")
}

func (s *Server) badRequestResponse(w http.ResponseWriter, r *http.Request, err error) {
	// s.logger.Warnf("bad request", "method", r.Method, "path", r.URL.Path, "error", err.Error())

	writeJSONError(w, http.StatusBadRequest, err.Error())
}

func (s *Server) multiStatusReponse(w http.ResponseWriter, r *http.Request, err error) {
	writeJSONError(w, http.StatusMultiStatus, err.Error())
}

func (s *Server) conflictResponse(w http.ResponseWriter, r *http.Request, err error) {
	// s.logger.Errorf("conflict response", "method", r.Method, "path", r.URL.Path, "error", err.Error())

	writeJSONError(w, http.StatusConflict, err.Error())
}

func (s *Server) notFoundResponse(w http.ResponseWriter, r *http.Request, err error) {
	// s.logger.Warnf("not found error", "method", r.Method, "path", r.URL.Path, "error", err.Error())

	writeJSONError(w, http.StatusNotFound, "not found")
}

func (s *Server) unauthorizedErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	// s.logger.Warnf("unauthorized error", "method", r.Method, "path", r.URL.Path, "error", err.Error())

	writeJSONError(w, http.StatusUnauthorized, "unauthorized")
}

func (s *Server) unauthorizedBasicErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	// s.logger.Warnf("unauthorized basic error", "method", r.Method, "path", r.URL.Path, "error", err.Error())

	w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)

	writeJSONError(w, http.StatusUnauthorized, "unauthorized")
}

func (s *Server) rateLimitExceededResponse(w http.ResponseWriter, r *http.Request, retryAfter string) {
	// s.logger.Warnw("rate limit exceeded", "method", r.Method, "path", r.URL.Path)

	w.Header().Set("Retry-After", retryAfter)

	writeJSONError(w, http.StatusTooManyRequests, "rate limit exceeded, retry after: "+retryAfter)
}
