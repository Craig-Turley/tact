package api

import (
	"log"
	"net/http"
	"time"
)

type Server struct {
	Addr string
}

func NewServer(addr string) *Server {
	return &Server{
		Addr: addr,
	}
}

func (s *Server) HandlePostEmail(w http.ResponseWriter, r *http.Request) {		
	log.Println("Handling POST email")
}

func (s *Server) HandlePostList(w http.ResponseWriter, r *http.Request) {		
	log.Println("Handling POST email")
}

func (s *Server) NewEmailMux() http.Handler {
	router := http.NewServeMux()

	router.HandleFunc("POST /new", s.HandlePostEmail)
	router.HandleFunc("POST /list", s.HandlePostList)
	
	return router
}

func (s *Server) Run() error {
	router := http.NewServeMux()
	apiRouter := http.NewServeMux()
	emailRouter := s.NewEmailMux()
	
	apiRouter.HandleFunc("/health", s.HandlehealthCheck)
	apiRouter.Handle("/email/", http.StripPrefix("/email", emailRouter))

	middleware := MiddlewareChain(
		Logging,
	)

	router.Handle("/api/v1/", http.StripPrefix("/api/v1", middleware(apiRouter)))

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

type Middleware func(h http.Handler) http.Handler

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		start := time.Now()	
		next.ServeHTTP(w, r)
		log.Printf(r.Method, r.URL.Path, time.Since(start))
	})
}

func MiddlewareChain(xs ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		for i := len(xs)-1; i >= 0; i-- {
			x := xs[i]
			next = x(next)
		}
		
		return next
	}	
}
