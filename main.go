package main

import (
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	// _ "github.com/mattn/go-sqlite3"
	"github.com/robfig/cron"
)

type Env struct {
	SqlitePath string
}

var APPENV = Env{
	SqlitePath: "./scheduling.db",
}

type JobRepo interface {
	CreateJob(job *Job) error
}

type CDCQueue interface {
	Listen() *Job
}

type Job struct {
	Id   int
	Name string `json:"Name"`
	Cron string `json:"Cron"`
}

func NewJob(name, cron string) *Job {
	return &Job{
		Name: name,
		Cron: cron,
	}
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

func (s *Server) handleCron(w http.ResponseWriter, r *http.Request) {
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

	s.JobRepo.CreateJob(&job)
}

func (s *Server) NewCronMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /cron", s.handleCron)

	return mux
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	cronMux := s.NewCronMux()

	mux.Handle("/v1/api/", http.StripPrefix("/v1/api", cronMux))

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

func (s *Scheduler) Run() {
	for {
		job := s.cdcQueue.Listen()
		if err := s.scheduleRepo.ScheduleJob(job); err != nil {
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
	runAt string
}

type ScheduleRepo interface {
	ScheduleJob(job *Job) error
	// GetJobsDueBefore(t time.Time) ([]*Job, error)
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

	runAt := schedule.Next(time.Now())
	t := runAt.UTC().Format("2006-01-02T15:04:05.000Z")
	s.store[job.Id] = &ScheduleDBEntry{job, t}

	return nil
}

func NewSqliteDb() *sql.DB {
	db, err := sql.Open("sqlite3", APPENV.SqlitePath)
	if err != nil {
		panic(err)
	}

	return db
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
			e.Pull()
		}
	}
}

func (e *Executor) Pull() {
}

func main() {
	jobRepo := NewDBWithCDC()
	queService := NewQueService(jobRepo)

	scheduleRepo := NewInMemoryScheduleRepo()
	scheduler := NewScheduler(jobRepo, scheduleRepo)
	go scheduler.Run()

	queService.Enqueue(NewJob("Say Hello", "* * * * * *"))
	queService.Enqueue(NewJob("Say Goodbye", "* * * * * *"))

	server := NewServer(jobRepo, ":8080")

	log.Println(server.Start())
}
