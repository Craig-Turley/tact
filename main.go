package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/Craig-Turley/task-scheduler.git/pkg/idgen"
	"github.com/Craig-Turley/task-scheduler.git/pkg/utils"
	"github.com/bwmarrin/snowflake"
	"github.com/cespare/xxhash/v2"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"github.com/robfig/cron"
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

type Job struct {
	Id         snowflake.ID `json:"job_id"`
	Name       string       `json:"name"`
	Cron       string       `json:"cron"`
	RetryLimit int          `json:"retry_limit"`
	Type       JobType      `json:"job_type"`
}

func NewJob(name, cron string, retryLimit int, jobType JobType) *Job {
	return &Job{
		Name:       name,
		Cron:       cron,
		RetryLimit: retryLimit,
		Type:       jobType,
	}
}

// Validates the job fields provided
func (j *Job) Validate() error {
	if len(j.Name) == 0 {
		return utils.NewError(utils.ERROR_JOB_NAME_NOT_PROVIDED)
	}

	if ok := j.Type.Valid(); !ok {
		return utils.NewError(utils.ERROR_JOB_TYPE_INVALID)
	}

	if j.RetryLimit <= 0 {
		return utils.NewError(utils.ERROR_INVALID_RETRY_LIMIT)
	}

	return nil
}

type JobService interface {
	// pass just the job the scheduler will take care of finding its next occurence
	CreateJob(job *Job) error
	GetJob(id snowflake.ID) (*Job, error)
}

type jobService struct {
	repo JobRepo
}

// jobService implements the JobService interface
// can pass it any repo that fits the JobRepo interface
func NewJobService(repo JobRepo) *jobService {
	return &jobService{repo: repo}
}

func (s *jobService) CreateJob(job *Job) error {
	if err := job.Validate(); err != nil {
		return err
	}

	_, err := cron.Parse(job.Cron)
	if err != nil {
		return err
	}

	job.Id = idgen.NewId()
	if err := s.repo.CreateJob(job); err != nil {
		return err
	}

	return nil
}

func (s *jobService) GetJob(id snowflake.ID) (*Job, error) {
	return s.repo.GetJob(id)
}

type JobRepo interface {
	CreateJob(job *Job) error
	GetJob(id snowflake.ID) (*Job, error)
}

type SqliteJobRepo struct {
	store   *sql.DB
	mu      sync.Mutex
	cdcChan chan *Job // not sure about this. supposed to mimic cdc but not sure if I want/need it for my 0 users
}

func NewSqliteJobRepo(db *sql.DB) *SqliteJobRepo {
	return &SqliteJobRepo{
		store:   db,
		mu:      sync.Mutex{},
		cdcChan: make(chan *Job),
	}
}

// CreateJob sets the id of the job passed in from autoincrementing table and returns an erro
func (s *SqliteJobRepo) CreateJob(job *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := "INSERT INTO jobs (id, name, cron, retry_limit, type) VALUES (?, ?, ?, ?, ?)"
	_, err := s.store.Exec(query, job.Id, job.Name, job.Cron, job.RetryLimit, job.Type)
	if err != nil {
		return err
	}

	// again, this is an optimization for approxomatley 0 users
	// and if this deadlocks...we're screwed
	// deadlock counter: 2
	// "why do I even have this?" counter: 2
	// s.cdcChan <- job

	return nil
}

// ahh yes, when this has a lot of history than say goodbye to your memory
func (s *SqliteJobRepo) GetJob(id snowflake.ID) (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	row := s.store.QueryRow("SELECT * FROM jobs WHERE id=?", id)
	if err := row.Err(); err != nil {
		return nil, err
	}

	var job Job
	if err := row.Scan(&job.Id, &job.Name, &job.Cron, &job.RetryLimit, &job.Type); err != nil {
		return nil, err
	}

	return &job, nil
}

// Using this as a type alias for an email string
// possibly might move this later
type Email string

// Using this as a type alias for an template string
// possibly might move this later
type Template string

type EmailData struct {
	JobId  snowflake.ID `json:"job_id"`
	ListId snowflake.ID `json:"list_id"`
}

type EmailService interface {
	CreateEmailJob(data *EmailData) error
	GetEmailJobData(jobId snowflake.ID) (*EmailData, error)
	GetEmailList(jobId snowflake.ID) ([]Email, error)
	GetTemplate(jobId snowflake.ID) (Template, error)
}

type emailService struct {
	repo          EmailRepo
	templateStore TemplateStore
}

func NewEmailService(repo EmailRepo, store TemplateStore) *emailService {
	return &emailService{
		repo:          repo,
		templateStore: store,
	}
}

func (s *emailService) CreateEmailJob(data *EmailData) error {
	return s.repo.SaveEmailData(data)
}

func (s *emailService) GetEmailJobData(jobId snowflake.ID) (*EmailData, error) {
	return s.repo.GetEmailData(jobId)
}

func (s *emailService) GetEmailList(jobId snowflake.ID) ([]Email, error) {
	return s.repo.GetEmailList(jobId)
}

func (s *emailService) GetTemplate(jobId snowflake.ID) (Template, error) {
	return s.templateStore.GetTemplate(jobId)
}

// TODO add context support
type EmailRepo interface {
	SaveEmailData(data *EmailData) error
	GetEmailData(jobId snowflake.ID) (*EmailData, error)
	GetEmailList(listId snowflake.ID) ([]Email, error)
}

// TODO add context support
type SqliteEmailRepo struct {
	store         *sql.DB
	templateStore TemplateStore
	mu            sync.Mutex
}

func NewSqliteEmailRepo(db *sql.DB) *SqliteEmailRepo {
	return &SqliteEmailRepo{
		store:         db,
		templateStore: NewLocalTemplateStore(os.Getenv("TEMPLATE_DIR")),
		mu:            sync.Mutex{},
	}
}

func (s *SqliteEmailRepo) SaveEmailData(data *EmailData) error {
	query := "INSERT INTO email_job_data (job_id, list_id) VALUES (?, ?)"
	_, err := s.store.Exec(query, data.JobId, data.ListId)
	return err
}

func (s *SqliteEmailRepo) GetEmailData(jobId snowflake.ID) (*EmailData, error) {
	var emailData EmailData
	row := s.store.QueryRow("SELECT * FROM email_job_data WHERE job_id = ?", jobId)
	if row.Err() != nil {
		return nil, row.Err()
	}

	err := row.Scan(&emailData.JobId, &emailData.ListId)
	if err != nil {
		return nil, err
	}

	return &emailData, err
}

//
// func (s *SqliteEmailRepo) GetListId(jobId int) (int, error) {
// 	s.mu.Lock()
// 	defer s.mu.Unlock()
//
// 	query := `SELECT list_id FROM email_job_data WHERE job_id=?`
//
// 	row := s.store.QueryRow(query, jobId)
// 	var listId int
// 	if err := row.Scan(&listId); err != nil {
// 		return -1, err
// 	}
//
// 	return listId, nil
// }

func (s *SqliteEmailRepo) GetEmailList(listId snowflake.ID) ([]Email, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
    SELECT ea.email
    FROM email_addresses AS ea
    JOIN subscriptions AS s ON s.email_address_id = ea.id
    WHERE s.email_list_id = ?;
  `

	rows, err := s.store.Query(query, listId)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var emails []Email
	for rows.Next() {
		var email string
		if err = rows.Scan(&email); err != nil {
			return nil, fmt.Errorf("row scan failed: %w", err)
		}
		emails = append(emails, Email(email))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return emails, nil
}

// TODO fix this
func (s *SqliteEmailRepo) GetTemplate(jobId snowflake.ID) (Template, error) {
	// s.mu.Lock()
	// defer s.mu.Unlock()
	//
	// row := s.store.QueryRow("SELECT path FROM email_template WHERE schedule_id=?", scheduleId)
	//
	// var path string
	// if err := row.Scan(&path); err != nil {
	// 	return "", err
	// }
	//
	// template, err := os.ReadFile("file.txt")
	// if err != nil {
	// 	return "", err
	// }
	//
	// return string(template), nil
	return s.templateStore.GetTemplate(jobId)
}

// TODO split this into seperate read and write groups for better concurrent access
func NewSqliteDb(path string) *sql.DB {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		panic(err)
	}

	return db
}

// returns the 64-bit xxhash digest of x with a zero seed
func HashXX(x snowflake.ID) uint64 {
	return xxhash.Sum64String(strconv.FormatInt(int64(x), 10))
}

// retuns the constructed path of given digest. starts with a /
func ConstructPath(digest uint64) string {
	hash := fmt.Sprintf("%x", digest)
	path := fmt.Sprintf("%s/%s/%s.html", hash[0:4], hash[4:8], hash[8:16])
	return path
}

type TemplateStore interface {
	SaveTemplate(jobId snowflake.ID, t string) error
	GetTemplate(jobId snowflake.ID) (Template, error)
}

// not using pointers for now since data doesnt need to persist. we will see about this later
type LocalTemplateStore struct {
	dir string
}

func NewLocalTemplateStore(dir string) LocalTemplateStore {
	return LocalTemplateStore{
		dir: dir,
	}
}

func (l LocalTemplateStore) SaveTemplate(jobId snowflake.ID, t string) error {
	path := fmt.Sprintf("%s/%s", l.dir, ConstructPath(HashXX(jobId)))

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	file.Write([]byte(t))

	return nil
}

func (l LocalTemplateStore) GetTemplate(jobId snowflake.ID) (Template, error) {
	path := fmt.Sprintf("%s/%s", l.dir, ConstructPath(HashXX(jobId)))

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return Template(data), nil
}

type JobEvent struct {
	Job
	ScheduleId snowflake.ID
}

type SchedulingService interface {
	ScheduleJob(jobId snowflake.ID, cronStr string) error
	GetJobsDueBefore(timeStamp string) ([]*JobEvent, error)
	UpdateJobStatus(jobId snowflake.ID, newStatus Status) error
}

type schedulingService struct {
	repo SchedulingRepo
}

func NewSchedulingService(repo SchedulingRepo) *schedulingService {
	return &schedulingService{
		repo: repo,
	}
}

func (s *schedulingService) ScheduleJob(jobId snowflake.ID, cronStr string) error {
	schedule, err := cron.Parse(cronStr)
	if err != nil {
		return err
	}

	runAt := schedule.Next(time.Now().UTC()).Format(utils.TIME_FORMAT) // next run time after current time
	data := NewScheduleData(idgen.NewId(), jobId, runAt, StatusScheduled)

	return s.repo.ScheduleEvent(data)
}

func (s *schedulingService) GetJobsDueBefore(timeStamp string) ([]*JobEvent, error) {
	if _, err := time.Parse(utils.TIME_FORMAT, timeStamp); err != nil {
		return nil, err
	}

	return s.repo.GetJobsDueBefore(timeStamp)
}

func (s *schedulingService) UpdateJobStatus(jobId snowflake.ID, status Status) error {
	return s.repo.UpdateJobStatus(jobId, status)
}

type ScheduleData struct {
	id     snowflake.ID
	jobId  snowflake.ID
	runAt  string
	status Status
}

func NewScheduleData(id, jobId snowflake.ID, runAt string, status Status) *ScheduleData {
	return &ScheduleData{
		id:     id,
		jobId:  jobId,
		runAt:  runAt,
		status: status,
	}
}

type SchedulingRepo interface {
	ScheduleEvent(event *ScheduleData) error
	GetJobsDueBefore(timeStamp string) ([]*JobEvent, error)
	UpdateJobStatus(jobId snowflake.ID, status Status) error
}

type SqliteSchedulingRepo struct {
	store *sql.DB
}

func NewSqliteSchedulingRepo(db *sql.DB) *SqliteSchedulingRepo {
	return &SqliteSchedulingRepo{
		store: db,
	}
}

func (s SqliteSchedulingRepo) ScheduleEvent(event *ScheduleData) error {
	query := "INSERT INTO scheduling (id, job_id, run_at, status) VALUES (?, ?, ?, ?)"
	_, err := s.store.Exec(query, event.id, event.jobId, event.runAt, event.status)
	if err != nil {
		return errors.New("Error inserting into table")
	}

	return nil
}

func (s SqliteSchedulingRepo) GetJobsDueBefore(timeStamp string) ([]*JobEvent, error) {
	query := "SELECT j.id, j.name, j.cron, j.retry_limit, j.type, s.id, AS schedule_id FROM jobs j JOIN scheduling s ON s.job_id=j.id WHERE s.run_at < ? AND s.status = ?"
	rows, err := s.store.Query(query, timeStamp, StatusScheduled)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*JobEvent

	for rows.Next() {
		var entry JobEvent
		if err := rows.Scan(&entry.Id, &entry.Name, &entry.Cron, &entry.RetryLimit, &entry.Type, &entry.ScheduleId); err != nil {
			return nil, err
		}

		entries = append(entries, &entry)
	}

	return entries, nil
}

func (s SqliteSchedulingRepo) UpdateJobStatus(id snowflake.ID, status Status) error {
	query := "UPDATE scheduling SET status = ? WHERE job_id = ?"
	_, err := s.store.Exec(query, status, id)

	// TODO handle the case where multiple rows are updated
	// should only be one
	//

	// if res.RowsAffected() != 1 {
	//
	//  }
	return err
}

// This is the service responsible for polling
// the scheduling table and shooting off workers
// for tasks that are due to run. It runs a ticker
// that polls the scheduler service for overdue jobs,
// spins up workers and those workers update the
// job status via the SchedulingService (ex Completed)
type ExecutorService interface {
	Start()
	Poll()
}

type executorService struct {
	Tick           time.Duration
	jobSrvc        JobService
	emailSrvc      EmailService
	schedulingSrvc SchedulingService
}

func NewExecutorService(tick time.Duration, jobSrvc JobService, emailSrvc EmailService, schedulingSrvc SchedulingService) *executorService {
	return &executorService{
		Tick:           tick,
		jobSrvc:        jobSrvc,
		emailSrvc:      emailSrvc,
		schedulingSrvc: schedulingSrvc,
	}
}

func (e *executorService) Start() {
	ticker := time.NewTicker(e.Tick)

	for {
		select {
		case <-ticker.C:
			e.Poll()
		}
	}
}

func (e *executorService) Poll() {
	time := time.Now().UTC().Format(utils.TIME_FORMAT)

	jobEvents, err := e.schedulingSrvc.GetJobsDueBefore(time)
	if err != nil {
		// TODO handle this this error
		// TODO this gets a double todo because it's important
		log.Println(utils.NewError("Error during executor polling: %e", err))
		return
	}

	for _, event := range jobEvents {
		log.Println("Working Job %d", event.ScheduleId)
	}
}

// Not sure about the arguments.
// Could optimize this by passing a whole
// job for the worker to use instead of
// making an extra call to the job service
type WorkerQue interface {
	Enque(e *JobEvent)
	Deque() *JobEvent
}

type LocalWorkerQue struct{}

var ALLOWED_TAGS = map[string]bool{
	"html":   true,
	"head":   true,
	"body":   true,
	"style":  true,
	"table":  true,
	"tr":     true,
	"td":     true,
	"th":     true,
	"thead":  true,
	"tbody":  true,
	"tfoot":  true,
	"div":    true,
	"span":   true,
	"p":      true,
	"br":     true,
	"hr":     true,
	"h1":     true,
	"h2":     true,
	"h3":     true,
	"h4":     true,
	"h5":     true,
	"h6":     true,
	"a":      true,
	"img":    true,
	"strong": true,
	"em":     true,
	"b":      true,
	"i":      true,
	"u":      true,
	"ul":     true,
	"ol":     true,
	"li":     true,
	"font":   true,
	"center": true,
	"meta":   true,
}

// TODO implement this
func SanitizeTemplate(t string) string {
	return t
}

func main() {
	godotenv.Load()

	utils.Assert("Time format must me set", len(utils.TIME_FORMAT) > 0, utils.TIME_FORMAT == os.Getenv("TIME_FORMAT"))

	nodeStr := os.Getenv("NODE_ID")
	node, err := strconv.ParseInt(nodeStr, 10, 64)
	if err != nil {
		panic("Couldn't initialize snowflake id node")
	}

	if err := idgen.Init(node); err != nil {
		log.Panicf("Error initializing snowflake node %s", err)
	}

	sqlite3db := NewSqliteDb(os.Getenv("SQLITE_DB_PATH"))

	emailRepo := NewSqliteEmailRepo(sqlite3db)
	templateStore := NewLocalTemplateStore(os.Getenv("TEMPLATE_DIR"))
	emailSrvc := NewEmailService(emailRepo, templateStore)

	jobRepo := NewSqliteJobRepo(sqlite3db)
	jobSrvc := NewJobService(jobRepo)

	// j := NewJob("test job", "* * * * * *", 3, TypeEmail)
	// jobSrvc.CreateJob(j)
	//
	// e := &EmailData{
	// 	JobId:  j.Id,
	// 	ListId: 1,
	// }
	//
	// emailSrvc.CreateEmailJob(e)

	schedulingRepo := NewSqliteSchedulingRepo(sqlite3db)
	schedulingSrvc := NewSchedulingService(schedulingRepo)

	// if err := schedulingService.ScheduleJob(j.Id, j.Cron); err != nil {
	// 	log.Println(err)
	// }

	tick, err := time.ParseDuration(os.Getenv("TICK"))
	if err != nil {
		panic(err)
	}

	executorSrvc := NewExecutorService(tick, jobSrvc, emailSrvc, schedulingSrvc)
	executorSrvc.Start()

	select {}
}
