package api

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/Craig-Turley/task-scheduler.git/internal/db"
	"github.com/Craig-Turley/task-scheduler.git/internal/repos"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/email"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/job"
	"github.com/Craig-Turley/task-scheduler.git/pkg/utils"
	"github.com/bwmarrin/snowflake"
)

type Server struct {
	Addr  string
	Store *repos.Storage
}

type ServerOpts struct{}

func NewServer(addr string) *Server {
	sqlite3db := db.NewSqliteDb("./scheduling.db")
	templateStorePath := os.Getenv("TEMPLATE_DIR")

	store := repos.NewSqliteStore(sqlite3db, templateStorePath)

	return &Server{
		Addr:  addr,
		Store: store,
	}
}

// NOTE: modifies data - saga pattern needed
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

	newJob, err := s.Store.Jobs.CreateJob(r.Context(), &jobInfo)
	if err != nil {
		s.internalServerError(w, r, nil)
		return
	}

	data, err := s.hydratePostJob(r.Context(), newJob, body)
	if err != nil {
		log.Println("Error in hydrateJob", err)
		s.Store.Jobs.DeleteJob(r.Context(), newJob.Id) // roll back this saga
		s.badRequestResponse(w, r, err)
		return
	}

	jobJSON, err := json.Marshal(newJob)
	if err != nil {
		s.internalServerError(w, r, nil)
		return
	}

	jobDataJSON, err := json.Marshal(data)
	if err != nil {
		s.internalServerError(w, r, nil)
		return
	}

	response := utils.MergeJson(jobJSON, jobDataJSON)

	if err := s.jsonResponse(w, http.StatusOK, response); err != nil {
		s.internalServerError(w, r, err)
		return
	}
}

func (s *Server) HandleGetJob(w http.ResponseWriter, r *http.Request) {
	parsedId, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.badRequestResponse(w, r, err)
		return
	}

	jobId := snowflake.ID(parsedId)

	j, err := s.Store.Jobs.GetJob(r.Context(), jobId)
	if err != nil {
		s.notFoundResponse(w, r, err)
		return
	}

	data, err := s.hydrateGetJob(r.Context(), j)
	if err != nil {
		s.notFoundResponse(w, r, err)
		return
	}

	jobJSON, err := json.Marshal(j)
	if err != nil {
		s.internalServerError(w, r, nil)
		return
	}

	jobDataJSON, err := json.Marshal(data)
	if err != nil {
		s.internalServerError(w, r, nil)
		return
	}

	response := utils.MergeJson(jobJSON, jobDataJSON)

	if err := s.jsonResponse(w, http.StatusOK, response); err != nil {
		s.internalServerError(w, r, err)
		return
	}
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
		if err := s.Store.Email.SaveEmailData(ctx, &emailData); err != nil {
			return nil, err
		}

		return emailData, nil
	}

	return nil, utils.NewError("Job type %d not supported in hydratePostJob", j.Type)
}

func (s *Server) hydrateGetJob(ctx context.Context, j *job.Job) (any, error) {
	switch j.Type {
	case job.TypeEmail:
		return s.Store.Email.GetEmailData(ctx, j.Id)
	}

	return nil, utils.NewError("Job type %d not supported in hydrateGetJob", j.Type)
}

// NOTE: modifies data
func (s *Server) HandlePostCreateList(w http.ResponseWriter, r *http.Request) {
	data := email.EmailListData{}
	if err := readJSON(w, r, &data); err != nil {
		s.badRequestResponse(w, r, err)
		return
	}

	newId, err := s.Store.Email.CreateEmailList(r.Context(), &data)
	if err != nil {
		s.badRequestResponse(w, r, nil)
		return
	}

	data.ListId = newId
	if err := s.jsonResponse(w, http.StatusOK, newId); err != nil {
		s.internalServerError(w, r, err)
		return
	}
}

type SubscribeRequest struct {
	Subscribers []*email.SubscriberInformation `json:"subscribers"`
}

const KEY_LIST_ID = "list_id"

func (s *Server) HandleGetList(w http.ResponseWriter, r *http.Request) {
	listIdParam := r.URL.Query().Get("list_id")
	if len(listIdParam) == 0 {
		s.badRequestResponse(w, r, utils.NewError("Error reading http path param"))
		return
	}

	listId, err := snowflake.ParseBase64(listIdParam)
	if err != nil {
		s.badRequestResponse(w, r, utils.NewError("Error reading http path param"))
		return
	}

	subscribers, err := s.Store.Email.GetEmailListSubscribers(r.Context(), listId)
	if err != nil {
		s.badRequestResponse(w, r, utils.NewError("Error getting list subscribers"))
		return
	}

	payload := SubscribeRequest{Subscribers: subscribers}
	bytes, err := json.Marshal(payload)
	if err != nil {
		s.internalServerError(w, r, nil)
		return
	}

	if err := s.jsonResponse(w, http.StatusOK, bytes); err != nil {
		s.internalServerError(w, r, err)
		return
	}
}

// NOTE: modifies data
func (s *Server) HandlePostSubscribeToList(w http.ResponseWriter, r *http.Request) {
	var req SubscribeRequest
	if err := readJSON(w, r, &req); err != nil {
		s.badRequestResponse(w, r, err)
		return
	}

	parsedId, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.badRequestResponse(w, r, utils.NewError("Invalid ID in request"))
		return
	}

	subs := []*email.SubscriberInformation{}
	for _, s := range req.Subscribers {
		subs = append(subs, email.NewSubscriberInformation(s.Id, s.FirstName, s.LastName, s.Email, s.ListId, s.IsSubscribed))
	}

	if err := s.Store.Email.AddToEmailList(r.Context(), snowflake.ID(parsedId), subs); err != nil {
		s.multiStatusReponse(w, r, err)
		return
	}

	if err := s.jsonResponse(w, http.StatusOK, nil); err != nil {
		s.internalServerError(w, r, err)
		return
	}
}

func (s *Server) HandleGetListSubscribers(w http.ResponseWriter, r *http.Request) {
	parsedId, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.badRequestResponse(w, r, utils.NewError("Error parsing list Id"))
		return
	}

	listId := snowflake.ID(parsedId)

	list, err := s.Store.Email.GetEmailListSubscribers(r.Context(), listId)
	if err != nil {
		s.notFoundResponse(w, r, err)
		return
	}

	if err := s.jsonResponse(w, http.StatusOK, list); err != nil {
		s.internalServerError(w, r, err)
		return
	}
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
