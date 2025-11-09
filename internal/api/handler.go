package api

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Craig-Turley/task-scheduler.git/internal/auth"
	"github.com/Craig-Turley/task-scheduler.git/internal/db"
	"github.com/Craig-Turley/task-scheduler.git/internal/repos"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/email"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/job"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/schedule"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/user"
	"github.com/Craig-Turley/task-scheduler.git/pkg/utils"
	"github.com/bwmarrin/snowflake"
	"github.com/golang-jwt/jwt/v5"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
)

type Server struct {
	Addr   string
	Origin string
	Store  *repos.Storage
}

func NewServer(addr string) *Server {
	sqlite3db := db.NewSqliteDb("./scheduling.db")
	templateStorePath := os.Getenv("TEMPLATE_DIR")
	origin := utils.Getenv("ORIGIN", "")
	if len(origin) == 0 {
		panic("ORIGIN not set")
	}

	store := repos.NewSqliteStore(sqlite3db, templateStorePath)

	return &Server{
		Addr:   addr,
		Origin: origin,
		Store:  store,
	}
}

// NOTE: modifies data - saga pattern needed
func (s *Server) HandlePostJob(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}

	// get the jobInfo out of the body and schedule data and then hydrate correct data
	var jobInfo job.Job
	if err := json.Unmarshal(body, &jobInfo); err != nil {
		log.Println("Error Parsing job", err)
		s.badRequestResponse(w, r, err)
		return
	}

	if err := jobInfo.Validate(); err != nil {
		log.Println("Invalid request body")
		s.badRequestResponse(w, r, err)
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

type HandleGetUserJobsResponse struct {
	JobIds []snowflake.ID `json:"ids"`
}

// TODO: verify this against the current session user id
func (s *Server) HandleGetUserJobs(w http.ResponseWriter, r *http.Request) {
	userId := r.PathValue("user_id")
	if len(userId) == 0 {
		s.badRequestResponse(w, r, utils.NewError("No User Id provided in the path"))
		return
	}

	jobTypeStr := r.URL.Query().Get("job_type")
	opts := &repos.GetUserJobOpts{}

	// TODO: clean this up
	// set the job type filter
	if len(jobTypeStr) != 0 {
		jobType, err := job.StringToJobType(jobTypeStr)
		if err != nil {
			s.badRequestResponse(w, r, err)
			return
		}

		opts.JobType = &jobType
	}

	jobs, err := s.Store.Jobs.GetUserJobs(r.Context(), userId, opts)
	if err != nil {
		s.notFoundResponse(w, r, err)
		return
	}

	if err := s.jsonResponse(w, http.StatusOK, jobs); err != nil {
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

func (s *Server) HandleGetEmailLists(w http.ResponseWriter, r *http.Request) {
	userId := r.PathValue("user_id")
	if len(userId) == 0 {
		s.badRequestResponse(w, r, utils.NewError("Error reading http path param"))
		return
	}

	data, err := s.Store.Email.GetEmailLists(r.Context(), userId)
	if err != nil {
		s.badRequestResponse(w, r, err)
		return
	}

	if err := s.jsonResponse(w, http.StatusOK, data); err != nil {
		s.internalServerError(w, r, err)
		return
	}
}

// type SubscribeRequest struct {
// 	Subscribers []*email.SubscriberInformation `json:"subscribers"`
// }

const KEY_LIST_ID = "list_id"

func (s *Server) HandleGetList(w http.ResponseWriter, r *http.Request) {
	listIdParam := r.PathValue("list_id")
	if len(listIdParam) == 0 {
		s.badRequestResponse(w, r, utils.NewError("Error reading http path param"))
		return
	}

	listId, err := snowflake.ParseString(listIdParam)
	if err != nil {
		s.badRequestResponse(w, r, utils.NewError("Error parsing id %s", listIdParam))
		return
	}

	subscribers, err := s.Store.Email.GetEmailListSubscribers(r.Context(), listId)
	if err != nil {
		s.badRequestResponse(w, r, utils.NewError("Error getting list subscribers"))
		return
	}

	if err := s.jsonResponse(w, http.StatusOK, subscribers); err != nil {
		s.internalServerError(w, r, err)
		return
	}
}

// NOTE: modifies data
func (s *Server) HandlePostSubscribeToList(w http.ResponseWriter, r *http.Request) {
	var req []*email.SubscriberInformation
	if err := readJSON(w, r, &req); err != nil {
		s.badRequestResponse(w, r, err)
		return
	}

	parsedId, err := strconv.ParseInt(r.PathValue("list_id"), 10, 64)
	if err != nil {
		s.badRequestResponse(w, r, utils.NewError("Invalid ID in request"))
		return
	}

	if err := s.Store.Email.AddToEmailList(r.Context(), snowflake.ID(parsedId), req); err != nil {
		s.multiStatusReponse(w, r, err)
		return
	}

	if err := s.jsonResponse(w, http.StatusOK, nil); err != nil {
		s.internalServerError(w, r, err)
		return
	}
}

func (s *Server) HandleGetListSubscribers(w http.ResponseWriter, r *http.Request) {
	parsedId, err := strconv.ParseInt(r.PathValue("list_id"), 10, 64)
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

func (s *Server) getUserAuthProvider(r *http.Request) (*http.Request, error) {
	provider := r.PathValue("provider")
	if len(provider) == 0 {
		return r, utils.NewError("Cannot parse auth provider")
	}

	return r.WithContext(context.WithValue(r.Context(), "provider", provider)), nil
}

func (s *Server) HandleGetAuth(w http.ResponseWriter, r *http.Request) {
	var err error
	r, err = s.getUserAuthProvider(r)
	if err != nil {
		s.badRequestResponse(w, r, utils.NewError("Invalid ID in request"))
		return
	}

	if user, err := gothic.CompleteUserAuth(w, r); err == nil {
		s.authorizeAndRedirectUser(user, w, r)
	} else {
		gothic.BeginAuthHandler(w, r)
	}
}

func (s *Server) authorizeAndRedirectUser(user goth.User, w http.ResponseWriter, r *http.Request) {
	claims := jwt.MapClaims{
		"sub":       user.UserID,
		"email":     user.Email,
		"name":      user.Name,
		"nick":      user.NickName,
		"avatar":    user.AvatarURL,
		"provider":  user.Provider,
		"issued_at": time.Now().Unix(),
		"exp":       time.Now().Add(24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(auth.JWTSecret))
	if err != nil {
		s.clientErrorRedirect(w, r, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:  auth.CookieName,
		Value: signedToken,
		Path:  "/",
		// Domain: omit on localhost
		MaxAge:   24 * 3600,
		HttpOnly: true,
		Secure:   false, // set true when serving over https
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, s.Origin, http.StatusFound) // 302
}

func (s *Server) HandleAuthCallback(w http.ResponseWriter, r *http.Request) {
	var err error
	r, err = s.getUserAuthProvider(r)
	if err != nil {
		s.badRequestResponse(w, r, utils.NewError("Invalid ID in request"))
		return
	}

	user, err := gothic.CompleteUserAuth(w, r)
	if err != nil {
		s.unauthorizedErrorResponse(w, r, err)
		return
	}

	s.authorizeAndRedirectUser(user, w, r)
}

func (s *Server) HandleGetUser(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(auth.CookieName)
	if err != nil || len(c.Value) == 0 {
		s.unauthorizedErrorResponse(w, r, err)
		return
	}

	token, err := jwt.Parse(c.Value, func(t *jwt.Token) (any, error) { return []byte(auth.JWTSecret), nil })
	if err != nil || !token.Valid {
		s.unauthorizedErrorResponse(w, r, err)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		s.unauthorizedErrorResponse(w, r, err)
		return
	}

	user := user.UserFromClaims(claims)
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) HandleGetLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	if err := gothic.Logout(w, r); err != nil {
		s.unauthorizedErrorResponse(w, r, err)
	}

	writeJSON(w, http.StatusOK, nil)
}

// TODO: test this
func (s *Server) HandlePostScheduleJob(w http.ResponseWriter, r *http.Request) {
	var schedulePayload schedule.ScheduleData
	if err := readJSON(w, r, &schedulePayload); err != nil {
		log.Println("Error Parsing schedule", err)
		s.badRequestResponse(w, r, err)
		return
	}

	t, err := time.Parse(utils.TIME_FORMAT, schedulePayload.RunAt)
	if err != nil {
		log.Println("Invalid time format", err)
		s.badRequestResponse(w, r, utils.NewError("Bad time format want %s", utils.TIME_FORMAT))
		return
	}

	// normalizing to utc
	schedulePayload.RunAt = t.UTC().Format(utils.TIME_FORMAT)

	scheduleData, err := s.Store.Schedule.ScheduleEvent(r.Context(), &schedulePayload)
	if err != nil {
		log.Println("Error Parsing schedule", err)
		s.internalServerError(w, r, err)
		return
	}

	if err := s.jsonResponse(w, http.StatusOK, scheduleData); err != nil {
		s.internalServerError(w, r, err)
		return
	}
}

func (s *Server) NewAuthRouter() http.Handler {
	router := http.NewServeMux()

	router.HandleFunc("GET /{provider}", s.HandleGetAuth)
	router.HandleFunc("GET /{provider}/callback", s.HandleAuthCallback)
	router.HandleFunc("GET /user", s.HandleGetUser)
	router.HandleFunc("POST /logout", s.HandleGetLogout)

	return router
}

func (s *Server) NewJobMux() http.Handler {
	router := http.NewServeMux()

	router.HandleFunc("POST /new", s.HandlePostJob)
	router.HandleFunc("GET /{id}", s.HandleGetJob)
	router.HandleFunc("GET /user/{user_id}", s.HandleGetUserJobs)

	return router
}

func (s *Server) NewEmailMux() http.Handler {
	router := http.NewServeMux()

	// TODO: organize endpoints and add an enpoint to get email data
	router.HandleFunc("POST /list", s.HandlePostCreateList)
	router.HandleFunc("GET /lists/{user_id}", s.HandleGetEmailLists)
	router.HandleFunc("GET /list/{list_id}", s.HandleGetList) // TODO: fix this
	router.HandleFunc("POST /list/{list_id}/subscribe", s.HandlePostSubscribeToList)
	router.HandleFunc("POST /template", s.HandlePostTemplate)

	return router
}

func (s *Server) NewScheduleMux() http.Handler {
	router := http.NewServeMux()

	router.HandleFunc("POST /new", s.HandlePostScheduleJob)

	return router
}

func (s *Server) Run() error {
	jobMux := s.NewJobMux()
	emailMux := s.NewEmailMux()
	scheduleMux := s.NewScheduleMux()
	// this is public - maybe more in the future not sure rn
	authMux := s.NewAuthRouter()

	// these are protected
	appMux := http.NewServeMux()
	appMux.Handle("/job/", http.StripPrefix("/job", jobMux))
	appMux.Handle("/email/", http.StripPrefix("/email", emailMux))
	appMux.Handle("/schedule/", http.StripPrefix("/schedule", scheduleMux))

	protectedMiddleware := MiddlewareChain(Logging, Authorization)
	publicMiddleware := MiddlewareChain(Logging)

	apiMux := http.NewServeMux()

	apiMux.Handle("/auth/", publicMiddleware(http.StripPrefix("/auth", authMux)))
	apiMux.HandleFunc("/health", s.HandlehealthCheck)

	apiMux.Handle("/", protectedMiddleware(appMux))

	root := http.NewServeMux()
	root.Handle("/api/v1/", http.StripPrefix("/api/v1", EnableCors(apiMux)))

	srv := http.Server{
		Addr:    s.Addr,
		Handler: root,
	}
	log.Printf("Server has started %s", s.Addr)
	return srv.ListenAndServe()
}

func (s *Server) HandlehealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello\n"))
}
