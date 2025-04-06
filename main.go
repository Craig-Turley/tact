package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
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

type EmailRepo interface {
	// Given job id (primary key of job table / job.Id), returns
	// the email list associated with the given
	GetEmailList(jobId int) ([]string, error)
	// Given scheduleId returns the associated template of the
	// given schedule id
	GetTemplate(scheduleId int) (string, error)
}

type SqliteEmailRepo struct {
	store *sql.DB
	mu    sync.Mutex
}

func NewSqliteEmailRepo(db *sql.DB) *SqliteEmailRepo {
	return &SqliteEmailRepo{
		store: db,
		mu:    sync.Mutex{},
	}
}

func (s *SqliteEmailRepo) GetEmailList(jobId int) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.store.Query("SELECT email FROM email_list WHERE job_id=?", jobId)
	if err != nil {
		return []string{}, err
	}
	defer rows.Close()

	emailList := []string{}
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, err
		}
		emailList = append(emailList, email)
	}

	return emailList, nil
}

func (s *SqliteEmailRepo) GetTemplate(scheduleId int) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	row := s.store.QueryRow("SELECT path FROM email_template WHERE schedule_id=?", scheduleId)

	var path string
	if err := row.Scan(&path); err != nil {
		return "", err
	}

	template, err := os.ReadFile("file.txt")
	if err != nil {
		return "", err
	}

	return string(template), nil
}

type Env struct {
	SqlitePath string
	TimeFormat string
	Duration   time.Duration
}

var (
	ERROR_INVALID_RETRY_LIMIT   = "Error invalid retry limit"
	ERROR_INVALID_JOB_TYPE      = "Error invalid job type"
	ERROR_JOB_NAME_NOT_PROVIDED = "Error job name not provided"
	ERROR_JOB_RUNNER_NOT_FOUND  = "Error associated job runner not found"
	ERROR_JOB_TYPE_MISMATCH     = "Error runner and job type mismatch. Got %d expected %d"
	ERROR_JOB_FAILED            = "Error job failed"
)

func NewError(template string, args ...any) error {
	if countFormats(template) != len(args) {
		return errors.New("Error: No information available - error generating message")
	}
	return errors.New(fmt.Sprintf(template, args...))
}

func countFormats(format string) int {
	re := regexp.MustCompile(`%[dfsuXxobegt]`)
	return len(re.FindAllString(format, -1))
}

var APPENV = Env{
	SqlitePath: "./scheduling.db",
	TimeFormat: "2006-01-02 15:04:05",
	Duration:   time.Duration(time.Second),
}

type JobRepo interface {
	CreateJob(job *Job) error
	GetJobs() ([]*Job, error)
}

type CDCQueue interface {
	Listen() *Job
}

type Job struct {
	Id         int     `json:"id"`
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
		return NewError(ERROR_JOB_NAME_NOT_PROVIDED)
	}

	if ok := j.Type.Valid(); !ok {
		return NewError(ERROR_INVALID_JOB_TYPE)
	}

	if j.RetryLimit <= 0 {
		return NewError(ERROR_INVALID_RETRY_LIMIT)
	}

	return nil
}

// in memory for testing
// type DBWithCDC struct {
// 	idInc   int
// 	store   map[int]*Job
// 	cdcChan chan *Job
// 	mu      sync.Mutex
// }
//
// func NewDBWithCDC() *DBWithCDC {
// 	return &DBWithCDC{
// 		idInc:   0,
// 		store:   make(map[int]*Job),
// 		cdcChan: make(chan *Job),
// 		mu:      sync.Mutex{},
// 	}
// }
//
// func (db *DBWithCDC) CreateJob(job *Job) error {
// 	db.mu.Lock()
// 	defer db.mu.Unlock()
// 	job.Id = db.idInc
// 	db.idInc += 1
//
// 	db.store[job.Id] = job
// 	db.cdcChan <- job
//
// 	return nil
// }
//
// func (db *DBWithCDC) GetJobs() ([]*Job, error) {
// 	res := make([]*Job, 0, len(db.store))
// 	for _, job := range db.store {
// 		res = append(res, job)
// 	}
// 	return res, nil
// }
//
// func (db *DBWithCDC) Listen() *Job {
// 	return <-db.cdcChan
// }

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

func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Alive"))
}

func (s *Server) NewCronMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /cron", s.handlePostCron)
	mux.HandleFunc("GET /cron", s.handleGetCrons)
	mux.HandleFunc("GET /health_check", s.healthCheck)

	return mux
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	cronMux := s.NewCronMux()

	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", cronMux))

	return http.ListenAndServe(s.Addr, mux)
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

// probably dont need this
// func (s *Scheduler) Schedule(job *Job) {
// 	log.Println("hello")
// 	// TODO handler error
// 	if err := s.scheduleRepo.ScheduleJob(job); err != nil {
// 		panic(err)
// 	}
// }

type ScheduleDBEntry struct {
	Job
	ScheduleId int
}

type ScheduleRepo interface {
	// pass just the job the scheduler will take care of finding its next occurence
	ScheduleJob(job *Job) error
	// pass a UTC time string in ISO-8601 format
	// this returns an entry with the event id and the job
	GetJobsDueBefore(timeString string) ([]*ScheduleDBEntry, error)
	// id is the id of the event not the job itself (this table holds past jobs)
	UpdateJobStatus(id int, status Status) error
}

// type InMemoryScheduleRepo struct {
// 	store map[int]*ScheduleDBEntry
// }
//
// func NewInMemoryScheduleRepo() *InMemoryScheduleRepo {
// 	return &InMemoryScheduleRepo{
// 		store: make(map[int]*ScheduleDBEntry),
// 	}
// }
//
// func (s *InMemoryScheduleRepo) ScheduleJob(job *Job) error {
// 	schedule, err := cron.Parse(job.Cron)
// 	if err != nil {
// 		return (err)
// 	}
//
// 	runAt := schedule.Next(time.Now().UTC()) // next run time after current time
// 	s.store[job.Id] = &ScheduleDBEntry{job, runAt}
//
// 	return nil
// }
//
// func (s *InMemoryScheduleRepo) GetJobsDueBefore(timeString string) ([]*Job, error) {
// 	t, err := time.Parse(APPENV.TimeFormat, timeString)
// 	if err != nil {
// 		// TODO handle this error
// 		return nil, err
// 	}
//
// 	var res []*Job
// 	for _, j := range s.store {
// 		if j.runAt.UTC().Before(t.UTC()) {
// 			res = append(res, j.Job)
// 		}
// 	}
//
// 	return res, nil
// }

// CreateJob(job *Job) error
// GetJobs() ([]*Job, error)
type SqliteJobRepo struct {
	store   *sql.DB
	mu      sync.Mutex
	cdcChan chan *Job
}

func NewSqliteJobRepo(db *sql.DB) *SqliteJobRepo {
	return &SqliteJobRepo{
		store:   db,
		mu:      sync.Mutex{},
		cdcChan: make(chan *Job),
	}
}

func (s *SqliteJobRepo) CreateJob(job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := "INSERT INTO jobs (name, cron, retry_limit, type) VALUES (?, ?, ?, ?)"
	result, err := s.store.Exec(query, job.Name, job.Cron, job.RetryLimit, job.Type)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	job.Id = int(id)
	s.cdcChan <- job

	return nil
}

func (s *SqliteJobRepo) GetJobs() ([]*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.store.Query("SELECT * FROM jobs")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := []*Job{}
	for rows.Next() {
		var job Job
		if err := rows.Scan(&job.Id, &job.Name, &job.Cron, &job.RetryLimit, &job.Type); err != nil {
			return nil, err
		}
		jobs = append(jobs, &job)
	}

	return jobs, nil
}

func (s *SqliteJobRepo) Listen() *Job {
	return <-s.cdcChan
}

type SqliteScheduleRepo struct {
	store *sql.DB
	mu    sync.Mutex
}

func NewSqliteScheduleRepo(db *sql.DB) *SqliteScheduleRepo {
	return &SqliteScheduleRepo{
		store: db,
		mu:    sync.Mutex{},
	}
}

func (s *SqliteScheduleRepo) ScheduleJob(job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	schedule, err := cron.Parse(job.Cron)
	if err != nil {
		return (err)
	}

	runAt := schedule.Next(time.Now().UTC()).Format(APPENV.TimeFormat) // next run time after current time
	query := "INSERT INTO scheduling (job_id, run_at, status) VALUES (?, ?, ?)"
	_, err = s.store.Exec(query, job.Id, runAt, StatusScheduled)
	if err != nil {
		return errors.New("Error inserting into table")
	}

	return nil
}

func (s *SqliteScheduleRepo) GetJobsDueBefore(timeString string) ([]*ScheduleDBEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := "SELECT j.*, s.id AS schedule_id FROM jobs j JOIN scheduling s ON s.job_id=j.id WHERE s.run_at < ? AND s.status = ?"
	rows, err := s.store.Query(query, timeString, StatusScheduled)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []*ScheduleDBEntry{}
	for rows.Next() {
		var entry ScheduleDBEntry
		if err := rows.Scan(&entry.Id, &entry.Name, &entry.Cron, &entry.RetryLimit, &entry.Type, &entry.ScheduleId); err != nil {
			return nil, err
		}
		entries = append(entries, &entry)
	}

	return entries, nil
}

func (s *SqliteScheduleRepo) UpdateJobStatus(id int, status Status) error {
	tx, err := s.store.Begin()
	if err != nil {
		tx.Rollback()
		return err
	}

	query := "UPDATE scheduling SET status = ? WHERE id = ?"
	stmt, err := tx.Prepare(query)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(id, status)
	if err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()
	return nil
}

type Executor struct {
	Tick         time.Duration
	scheduleRepo ScheduleRepo
	emailRepo    EmailRepo
}

func NewExecutor(tick time.Duration, db *sql.DB) *Executor {
	return &Executor{
		Tick:         tick,
		scheduleRepo: NewSqliteScheduleRepo(db),
		emailRepo:    NewSqliteEmailRepo(db),
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

	events, err := e.scheduleRepo.GetJobsDueBefore(t)
	if err != nil {
		// TODO properly log this
		// maybe consider fail threshold and if it passes than send an alert
		log.Println(err)
		return
	}

	log.Printf("%d events found before %s", len(events), t)

	for _, event := range events {
		go e.Worker(event)
	}
}

func (e *Executor) Worker(entry *ScheduleDBEntry) {
	log.Printf("Doing %s entry %s with id %d retry limit %d", JobTypeToString(entry.Type), entry.Name, entry.Id, entry.RetryLimit)

	runner, err := e.GetRunner(entry.Type)
	if err != nil {
		// TODO handle this error
		log.Printf("Failed to fetch runner for job type %s", JobTypeToString(entry.Type))
	}

	for i := range entry.RetryLimit {
		err = runner(&entry.Job)
		if err != nil {
			log.Printf("Job with Id %d failed on attempt %d", entry.Id, i)
			continue
		}
		break
	}

	if err != nil {
		// TODO HANDLE THE ERROR RETURNED HERE
		if err = e.scheduleRepo.UpdateJobStatus(entry.Id, StatusFailed); err != nil {
			// TODO configure db down alerts
		}
	} else {
		// TODO HANDLE THE ERROR RETURNED HERE
		if err = e.scheduleRepo.UpdateJobStatus(entry.Id, StatusSuccess); err != nil {
			// TODO configure db down alerts
		}
	}
}

type JobRunner func(job *Job) error

func (e *Executor) GetRunner(t JobType) (JobRunner, error) {
	switch t {
	case TypeCustom:
		return e.CustomRunner, nil
	case TypeEmail:
		return e.EmailRunner, nil
	case TypeDiscord:
		return e.DiscordRunner, nil
	case TypeSlack:
		return e.SlackRunner, nil
	}

	return nil, NewError(ERROR_JOB_RUNNER_NOT_FOUND)
}

func (e *Executor) CustomRunner(job *Job) error {
	return nil
}

func (e *Executor) EmailRunner(job *Job) error {
	if job.Type != TypeEmail {
		return NewError(ERROR_JOB_TYPE_MISMATCH, job.Type, TypeEmail)
	}

	emailList, err := e.emailRepo.GetEmailList(job.Id)
	if err != nil {
		return err
	}

	for _, email := range emailList {
		log.Printf("Sending email to %s", email)
	}

	return nil
}

func (e *Executor) DiscordRunner(job *Job) error {
	return nil
}

func (e *Executor) SlackRunner(job *Job) error {
	return nil
}

// TODO split this into seperate read and write groups for better concurrent access
func NewSqliteDb() *sql.DB {
	db, err := sql.Open("sqlite3", APPENV.SqlitePath)
	if err != nil {
		panic(err)
	}

	return db
}

func main() {
	sqlite3db := NewSqliteDb()

	jobRepo := NewSqliteJobRepo(sqlite3db)
	scheduleRepo := NewSqliteScheduleRepo(sqlite3db)

	scheduler := NewScheduler(jobRepo, scheduleRepo)
	go scheduler.Start()

	executor := NewExecutor(APPENV.Duration, sqlite3db)
	go executor.Start()

	server := NewServer(jobRepo, ":8080")

	log.Println(server.Start())
}
