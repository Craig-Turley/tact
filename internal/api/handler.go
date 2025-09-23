package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Craig-Turley/task-scheduler.git/internal/db"
	"github.com/Craig-Turley/task-scheduler.git/internal/repos"
	"github.com/Craig-Turley/task-scheduler.git/internal/services"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/email"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/job"
	"github.com/Craig-Turley/task-scheduler.git/pkg/utils"
	"github.com/bwmarrin/snowflake"
)

type Server struct {
	Addr         string
	jobService   services.JobService
	emailService services.EmailService
}

func NewServer(addr string) *Server {
	log.Println(os.LookupEnv("SQLITE_DB_PATH"))
	sqlite3db := db.NewSqliteDb("./scheduling.db")

	jobRepo := repos.NewSqliteJobRepo(sqlite3db)
	emailRepo := repos.NewSqliteEmailRepo(sqlite3db)
	templateStore := repos.NewLocalTemplateStore(os.Getenv("TEMPLATE_DIR"))

	return &Server{
		Addr:         addr,
		jobService:   services.NewJobService(jobRepo),
		emailService: services.NewEmailService(emailRepo, templateStore),
	}
}

func (s *Server) HandlePostJob(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}

	// get the jobInfo out of the body and hydrate correct data
	var jobInfo job.Job
	if err := json.Unmarshal(body, &jobInfo); err != nil {
		log.Println("Error Parsing job", err)
		http.Error(w, "Malformed Request. Error parsing job details", http.StatusBadRequest)
		return
	}

	newJob, err := s.jobService.CreateJob(r.Context(), &jobInfo)
	if err != nil {
		http.Error(w, "Error saving job", http.StatusInternalServerError)
		return
	}

	log.Println("New Job ID", newJob.Id)

	data, err := s.hydratePostJob(r.Context(), newJob, body)
	if err != nil {
		log.Println("Error in hydrateJob", err)
		http.Error(w, fmt.Sprintf("Malformed Request. Error parsing details of job type %d", newJob.Type), http.StatusBadRequest)
		return
	}

	jobJSON, err := json.Marshal(newJob)
	if err != nil {
		http.Error(w, "failed to marshal job data", http.StatusInternalServerError)
		return
	}

	jobDataJSON, err := json.Marshal(data)
	if err != nil {
		http.Error(w, "failed to marshal job data", http.StatusInternalServerError)
		return
	}

	response := utils.MergeJson(jobJSON, jobDataJSON)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

func (s *Server) HandleGetJob(w http.ResponseWriter, r *http.Request) {
	parsedId, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID in request", http.StatusBadRequest)
		return
	}

	jobId := snowflake.ID(parsedId)

	j, err := s.jobService.GetJob(r.Context(), jobId)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting job with ID %d, %s", jobId, err.Error()), http.StatusNotFound)
		return
	}

	content, err := json.Marshal(j)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(content)
}

func (s *Server) hydratePostJob(ctx context.Context, j *job.Job, data []byte) (any, error) {
	switch j.Type {
	case job.TypeEmail:
		var emailData email.EmailData
		err := json.Unmarshal(data, &emailData)
		if err != nil {
			return nil, err
		}

		emailData.JobId = j.Id
		if err := s.emailService.CreateEmailJob(ctx, &emailData); err != nil {
			return nil, err
		}

		return emailData, nil
	}

	return nil, utils.NewError("Job type %d not supported in hydratePostJob", j.Type)
}

func (s *Server) hydrateGetJob(ctx context.Context, j *job.Job) (any, error) {
	switch j.Type {
	case job.TypeEmail:
		return s.emailService.GetEmailJobData(ctx, j.Id)
	}

	return nil, utils.NewError("Job type %d not supported in hydrateGetJob", j.Type)
}

func (s *Server) HandlePostCreateList(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}

	data := &email.EmailListData{}
	if err := json.Unmarshal(body, data); err != nil {
		http.Error(w, "Error parsing request body", http.StatusBadRequest)
		return
	}

	newId, err := s.emailService.CreateEmailList(r.Context(), data)
	if err != nil {
		http.Error(w, "Error creating new email list", http.StatusBadRequest)
		return
	}

	data.ListId = newId
	w.WriteHeader(http.StatusOK)
}

type SubscribeRequest struct {
	Subscribers []*email.SubscriberInformation `json:"subscribers"`
}

const KEY_LIST_ID = "list_id"

func (s *Server) HandleGetList(w http.ResponseWriter, r *http.Request) {
	listIdParam := r.URL.Query().Get("list_id")
	if len(listIdParam) == 0 {
		http.Error(w, "Error reading http path param", http.StatusBadRequest)
		return
	}

	listId, err := snowflake.ParseBase64(listIdParam)
	if err != nil {
		http.Error(w, "Error reading http path param", http.StatusBadRequest)
		return
	}

	subscribers, err := s.emailService.GetEmailListSubscribers(r.Context(), listId)
	if err != nil {
		http.Error(w, "Error getting list subscribers", http.StatusBadRequest)
		return
	}

	payload := SubscribeRequest{Subscribers: subscribers}
	bytes, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(bytes)
}

func (s *Server) HandlePostSubscribeToList(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}

	parsedId, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID in request", http.StatusBadRequest)
		return
	}

	var req SubscribeRequest
	if err := json.Unmarshal(body, &req.Subscribers); err != nil {
		log.Println("Error Parsing subscriber list", err)
		http.Error(w, "Malformed Request. Error parsing subscriber details", http.StatusBadRequest)
		return
	}

	subs := []*email.SubscriberInformation{}
	for _, s := range req.Subscribers {
		subs = append(subs, email.NewSubscriberInformation(s.Id, s.FirstName, s.LastName, s.Email, s.ListId, s.IsSubscribed))
	}

	s.emailService.AddToEmailList(r.Context(), snowflake.ID(parsedId), subs)
}

func (s *Server) HandleGetListSubscribers(w http.ResponseWriter, r *http.Request) {
	parsedId, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID in request", http.StatusBadRequest)
		return
	}

	listId := snowflake.ID(parsedId)

	list, err := s.emailService.GetEmailListSubscribers(r.Context(), listId)
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
	router.HandleFunc("GET /{id}", s.HandleGetJob)

	return router
}

func (s *Server) NewEmailMux() http.Handler {
	router := http.NewServeMux()

	router.HandleFunc("POST /list", s.HandlePostCreateList)
	router.HandleFunc("GET /list", s.HandleGetList)
	router.HandleFunc("POST /list/{id}/subscribe", s.HandlePostSubscribeToList)
	router.HandleFunc("GET /list/{id}", s.HandleGetListSubscribers)
	router.HandleFunc("POST /template", s.HandlePostTemplate)

	return router
}

func (s *Server) Run() error {
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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf(r.Method, r.URL.Path, time.Since(start))
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
