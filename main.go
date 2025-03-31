package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	// _ "github.com/mattn/go-sqlite3"
	"github.com/robfig/cron"
)

type JobType uint8

const (
	TypeStart JobType = iota
	// custom scripts
	TypeCustom
	// automated messaging
	TypeEmail
	TypeSlack
	TypeDiscord

	TypeEnd
)

type Status uint8

const (
	StatusStart Status = iota
	StatusScheduled
	StatusFailed
	StatusRunning
	StatusSuccess
	StatusEnd
)

func (t JobType) Valid() bool {
	return TypeStart < t && t < TypeEnd
}

func JobTypeToString(t JobType) string {
	switch t {
	case TypeCustom:
		return "Custom"
	case TypeEmail:
		return "Email"
	case TypeSlack:
		return "Slack"
	case TypeDiscord:
		return "Discord"
	}

	return "not found"
}

type JobRunner func(job *Job) error

func GetRunner(t JobType) (JobRunner, error) {
	switch t {
	case TypeEmail:
		return EmailRunner, nil
	case TypeDiscord:
		return DiscordRunner, nil
	case TypeSlack:
		return SlackRunner, nil
	}

	return nil, ERROR_JOB_RUNNER_NOT_FOUND
}

func EmailRunner(job *Job) error {
	return nil
}

func DiscordRunner(job *Job) error {
	return nil
}

func SlackRunner(job *Job) error {
	return nil
}

type Env struct {
	SqlitePath string
	TimeFormat string
}

var (
	ERROR_INVALID_RETRY_LIMIT   = errors.New("Error invalid retry limit")
	ERROR_INVALID_JOB_TYPE      = errors.New("Error invalid job type")
	ERROR_JOB_NAME_NOT_PROVIDED = errors.New("Error job name not provided")
	ERROR_JOB_RUNNER_NOT_FOUND  = errors.New("Error associated job runner not found")
)

var APPENV = Env{
	SqlitePath: "./scheduling.db",
	TimeFormat: "2006-01-02T15:04:05.000Z",
}

type JobRepo interface {
	CreateJob(job *Job) error
	GetJobs() ([]*Job, error)
}

type CDCQueue interface {
	Listen() *Job
}

type Job struct {
	Id         int     `josn:"id"`
	Name       string  `json:"name"`
	Cron       string  `json:"cron"`
	RetryLimit int     `json:"retry_limit"`
	Type       JobType `json:"job_type"`
}

func NewJob(name, cron string, retryLimit int, jobType JobType) *Job {
	return &Job{
		Name:       name,
		Cron:       cron,
		RetryLimit: retryLimit,
		Type:       jobType,
	}
}

func (j *Job) Validate() error {
	if len(j.Name) == 0 {
		return ERROR_JOB_NAME_NOT_PROVIDED
	}

	if ok := j.Type.Valid(); !ok {
		return ERROR_INVALID_JOB_TYPE
	}

	if j.RetryLimit <= 0 {
		return ERROR_INVALID_RETRY_LIMIT
	}

	return nil
}

type DBWithCDC struct {
	idInc   int
	store   map[int]*Job
	cdcChan chan *Job
}

func NewDBWithCDC() *DBWithCDC {
	return &DBWithCDC{
		idInc:   0,
		store:   make(map[int]*Job),
		cdcChan: make(chan *Job),
	}
}

func (db *DBWithCDC) CreateJob(job *Job) error {
	job.Id = db.idInc
	db.idInc += 1

	db.store[job.Id] = job
	db.cdcChan <- job

	return nil
}

func (db *DBWithCDC) GetJobs() ([]*Job, error) {
	res := make([]*Job, 0, len(db.store))
	for _, job := range db.store {
		res = append(res, job)
	}
	return res, nil
}

func (db *DBWithCDC) Listen() *Job {
	return <-db.cdcChan
}

type Server struct {
	JobRepo JobRepo
	Addr    string
}

func NewServer(jobRepo JobRepo, addr string) *Server {
	return &Server{
		Addr:    addr,
		JobRepo: jobRepo,
	}
}

func (s *Server) handlePostCron(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		http.Error(w, "Malformed Request", http.StatusBadRequest)
		return
	}

	var job Job
	err = json.Unmarshal(body, &job)
	if err != nil {
		http.Error(w, "Malformed Request", http.StatusBadRequest)
		return
	}

	// TODO turn into middleware
	if err = job.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.JobRepo.CreateJob(&job)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Success"))
}

func (s *Server) handleGetCrons(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.JobRepo.GetJobs()
	if err != nil {
		http.Error(w, "Error getting jobs", http.StatusInternalServerError)
		return
	}

	j, err := json.Marshal(jobs)
	if err != nil {
		http.Error(w, "Error sending jobs", http.StatusInternalServerError)
	}

	w.WriteHeader(http.StatusOK)
	w.Write(j)
}

func (s *Server) NewCronMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /cron", s.handlePostCron)
	mux.HandleFunc("GET /cron", s.handleGetCrons)

	return mux
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	cronMux := s.NewCronMux()

	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", cronMux))

	return http.ListenAndServe(s.Addr, mux)
}

type QueService struct {
	JobRepo JobRepo
}

func NewQueService(jobRepo JobRepo) *QueService {
	return &QueService{
		JobRepo: jobRepo,
	}
}

func (q *QueService) Enqueue(job *Job) error {
	if err := q.JobRepo.CreateJob(job); err != nil {
		return err
	}

	return nil
}

type Scheduler struct {
	cdcQueue     CDCQueue
	scheduleRepo ScheduleRepo
}

func NewScheduler(que CDCQueue, scheduleRepo ScheduleRepo) *Scheduler {
	return &Scheduler{
		cdcQueue:     que,
		scheduleRepo: scheduleRepo,
	}
}

func (s *Scheduler) Start() {
	for {
		job := s.cdcQueue.Listen()
		if err := s.scheduleRepo.ScheduleJob(job); err != nil {
			// TODO handle this error
			panic(err)
		}
	}
}

func (s *Scheduler) Schedule(job *Job) {
	// TODO handler error
	if err := s.scheduleRepo.ScheduleJob(job); err != nil {
		panic(err)
	}
}

type ScheduleDBEntry struct {
	*Job
	runAt time.Time
}

type ScheduleRepo interface {
	ScheduleJob(job *Job) error
	GetJobsDueBefore(timeString string) ([]*Job, error)
}

type InMemoryScheduleRepo struct {
	store map[int]*ScheduleDBEntry
}

func NewInMemoryScheduleRepo() *InMemoryScheduleRepo {
	return &InMemoryScheduleRepo{
		store: make(map[int]*ScheduleDBEntry),
	}
}

func (s *InMemoryScheduleRepo) ScheduleJob(job *Job) error {
	schedule, err := cron.Parse(job.Cron)
	if err != nil {
		return (err)
	}

	runAt := schedule.Next(time.Now().UTC()) // next run time after current time
	s.store[job.Id] = &ScheduleDBEntry{job, runAt}

	return nil
}

func (s *InMemoryScheduleRepo) GetJobsDueBefore(timeString string) ([]*Job, error) {
	t, err := time.Parse(APPENV.TimeFormat, timeString)
	if err != nil {
		// TODO handle this error
		return nil, err
	}

	var res []*Job
	for _, j := range s.store {
		if j.runAt.UTC().Before(t.UTC()) {
			res = append(res, j.Job)
		}
	}

	return res, nil
}

type SqliteScheduleRepo struct {
	store *sql.DB
}

func NewSqliteScheduleRepo(db *sql.DB) *SqliteScheduleRepo {
	return &SqliteScheduleRepo{
		store: db,
	}
}

func (s *SqliteScheduleRepo) ScheduleJob(job *Job) error {
	schedule, err := cron.Parse(job.Cron)
	if err != nil {
		return (err)
	}

	runAt := schedule.Next(time.Now().UTC()) // next run time after current time
	query := fmt.Sprintf("INSERT INTO scheduling (id, run_at) VALUES (%d, %s)", job.Id, runAt)
	_, err = s.store.Exec(query)
	if err != nil {
		return errors.New("Error inserting into table")
	}

	return nil
}

type Executor struct {
	Tick         time.Duration
	scheduleRepo ScheduleRepo
}

func NewExecutor(tick time.Duration, scheduleRepo ScheduleRepo) *Executor {
	return &Executor{
		Tick:         tick,
		scheduleRepo: scheduleRepo,
	}
}

func (e *Executor) Start() {
	ticker := time.NewTicker(e.Tick)

	for {
		select {
		case <-ticker.C:
			e.Poll()
		}
	}
}

func (e *Executor) Poll() {
	t := time.Now().UTC().Format(APPENV.TimeFormat)

	jobs, err := e.scheduleRepo.GetJobsDueBefore(t)
	if err != nil {
		// TODO handle this error
		panic(err)
	}

	log.Printf("%d jobs found before %s", len(jobs), t)

	for _, job := range jobs {
		go e.Worker(job)
	}
}

func (e *Executor) Worker(job *Job) {
	log.Printf("Doing %s job %s with id %d retry limit %d", JobTypeToString(job.Type), job.Name, job.Id, job.RetryLimit)
	var err error

	runner, err := GetRunner(job.Type)
	if err != nil {
		// TODO handle this error
		panic(err)
	}

	for i := range job.RetryLimit {
		err = runner(job)
		if err != nil {
			log.Printf("Job with Id %d failed on attempt %d", job.Id, i)
			continue
		}
		break
	}

	if err == nil {
		log.Printf("Job with Id %d succeeded", job.Id)
	}
}

func NewSqliteDb() *sql.DB {
	db, err := sql.Open("sqlite3", APPENV.SqlitePath)
	if err != nil {
		panic(err)
	}

	return db
}

func main() {
	jobRepo := NewDBWithCDC()
	queService := NewQueService(jobRepo)

	scheduleRepo := NewInMemoryScheduleRepo()
	scheduler := NewScheduler(jobRepo, scheduleRepo)
	go scheduler.Start()

	executor := NewExecutor(time.Duration(time.Second), scheduleRepo)
	go executor.Start()

	queService.Enqueue(NewJob("Say Hello", "* * * * * *", 3, TypeEmail))
	queService.Enqueue(NewJob("Say Goodbye", "* * * * * *", 3, TypeDiscord))
	queService.Enqueue(NewJob("Fail", "* * * * * *", 3, TypeCustom))

	server := NewServer(jobRepo, ":8080")

	log.Println(server.Start())
}
