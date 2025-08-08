package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Craig-Turley/task-scheduler.git/internal/db"
	"github.com/Craig-Turley/task-scheduler.git/internal/repos"
	"github.com/Craig-Turley/task-scheduler.git/internal/services"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/email"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/job"
	"github.com/bwmarrin/snowflake"
)

type Server struct {
	Addr string
	jobService services.JobService
	emailService services.EmailService
}

func NewServer(addr string) *Server {
	// TODO fix the env not working 
	log.Println(os.LookupEnv("SQLITE_DB_PATH"))
	sqlite3db := db.NewSqliteDb("./scheduling.db")

	jobRepo := repos.NewSqliteJobRepo(sqlite3db)
	emailRepo := repos.NewSqliteEmailRepo(sqlite3db)
	templateStore := repos.NewLocalTemplateStore(os.Getenv("TEMPLATE_DIR"))

	return &Server{
		Addr: addr,
		jobService: services.NewJobService(jobRepo),
		emailService: services.NewEmailService(emailRepo, templateStore),
	}
}

func (s *Server) HandlePostJob(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return 
	}

	// get the job out of the body and hydrate correct data
	var job job.Job
	if err := json.Unmarshal(body, &job); err != nil {
		log.Println("Error Parsing job", err)
		http.Error(w, "Malformed Request. Error parsing job details", http.StatusBadRequest)
		return
	}

	if err := s.jobService.CreateJob(&job); err != nil {
		log.Println("Error in hydrateJob", err)
		http.Error(w, "Error saving job", http.StatusInternalServerError)
		return
	}
	
	if err := s.hydrateJob(job.Type, body); err != nil {
		log.Println("Error in hydrateJob", err)
		http.Error(w, fmt.Sprintf("Malformed Request. Error parsing details of job type %d", job.Type), http.StatusBadRequest)
		return
	}
	
	w.WriteHeader(http.StatusOK)
}

func (s *Server) HandleGetJob(w http.ResponseWriter, r *http.Request) {
	// TODO find a way to get the jobId out of the URL
	// currently looks like this /job/:job-id
	jobId := snowflake.ID(1927946547291492352)
	j, err := s.jobService.GetJob(jobId)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting job with ID %d, %s", jobId, err.Error()), http.StatusNotFound)	
	}

	content, err := json.Marshal(j)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}

	w.WriteHeader(http.StatusOK)
	w.Write(content)
}

func (s *Server) hydrateJob(t job.JobType, data []byte) error {
	switch(t) {
	case job.TypeEmail:
		var emailData email.EmailData
		err := json.Unmarshal(data, &emailData)
		if err != nil {
			return err
		}
	
		log.Println("Saving the email job")
		// return s.emailService.CreateEmailJob(&emailData)
		return nil
	}	

	return nil
}

func (s *Server) HandlePostList(w http.ResponseWriter, r *http.Request) {
	// body, err := io.ReadAll(r.Body)
	// if err != nil {
	// 	http.Error(w, "Error reading request body", http.StatusBadRequest)
	// 	return 
	// }
	
	log.Println("Handling Post List")	
}

func (s *Server) HandleGetList(w http.ResponseWriter, r *http.Request) {
	listId := snowflake.ID(123)
	list, err := s.emailService.GetEmailList(listId)
	if err != nil {
		log.Printf("Error getting list of id %d", listId)
		http.Error(w, "Error getting list of id %d", http.StatusNotFound)
		return
	}

	contents, err := json.Marshal(list)
	if err != nil {
		log.Println("Error marshalling list data")
		http.Error(w, "Internal Server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(contents)
}

func (s *Server) HandlePostTemplate(w http.ResponseWriter, r *http.Request) {
	log.Println("Handling POST template")
}

func (s *Server) NewJobMux() http.Handler {
	router := http.NewServeMux()

	router.HandleFunc("POST /new", s.HandlePostJob)
	// TODO make this path parameter
	router.HandleFunc("GET /1", s.HandleGetJob)

	return router
}


func (s *Server) NewEmailMux() http.Handler {
	router := http.NewServeMux()

	router.HandleFunc("POST /list", s.HandlePostList)
	// TOOD make this a path parameter
	router.HandleFunc("GET /list/1", s.HandleGetList)
	router.HandleFunc("POST /template", s.HandlePostTemplate)
	
	return router
}

func (s *Server) Run() error {
	//TODO add asserts for the services. these cannot be nil...obviously

	router := http.NewServeMux()
	apiRouter := http.NewServeMux()
	jobRouter := s.NewJobMux()
	emailRouter := s.NewEmailMux()

	apiRouter.HandleFunc("/health", s.HandlehealthCheck)
	apiRouter.Handle("/job/", http.StripPrefix("/job", jobRouter))	
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
