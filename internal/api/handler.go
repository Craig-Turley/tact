package api

import (
	"log"
	"net/http"
)

type Server struct {
	Addr string
}

func NewServer(addr string) *Server {
	return &Server{
		Addr: addr,
	}
}

func (s *Server) Run() error {
	router := http.NewServeMux()

	router.HandleFunc("/health", s.HandlehealthCheck)

	server := http.Server{
		Addr:    s.Addr,
		Handler: router,
	}

	log.Printf("Server has started %s", s.Addr)

	return server.ListenAndServe()
}

func (s *Server) HandlehealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello\n"))
}
