package main

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

type JobStatus int

const (
	INPROGRESS JobStatus = iota
	SUCCESS
	FAILURE
)

type Database interface {
	GetJobs() ([]Job, error)
	UpdateStatus(id int, status JobStatus) error
}

type JobQueue interface {
	PushJob(job Job)
	PushJobs(jobs []Job)
	PullJob() <-chan Job
}

type ChanQueue chan Job

func (cq ChanQueue) PushJob(job Job) {
	cq <- job
}

func (cq ChanQueue) PushJobs(jobs []Job) {
	for _, job := range jobs {
		cq <- job
	}
}

func (cq ChanQueue) PullJob() <-chan Job {
	return cq
}

func NewChanQueue() ChanQueue {
	return make(ChanQueue)
}

type JobHandler func() error

type Job struct {
	Id      int
	Runat   time.Time
	Name    string
	Cron    string
	Handler JobHandler
	// Retries int
	// Timeout time.Time

	// this is mainly for the user
	LastRun time.Time
	Status  JobStatus
}

func SayHello() error {
	log.Println("Hello")
	return errors.New("Uhoh, we have a failure")
}

func SayGoodbye() error {
	log.Println("Goodbye")
	return nil
}

type JobDB struct {
	mu    sync.Mutex
	store []Job
}

func (d *JobDB) GetJobs() ([]Job, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.store, nil
}

func (d *JobDB) UpdateStatus(id int, status JobStatus) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	for i := range d.store {
		if d.store[i].Id == id {
			d.store[i].Status = status
			return nil
		}
	}

	return errors.New("Job not found")
}

func NewJobDB() *JobDB {
	return &JobDB{
		store: []Job{
			{1, time.Now().Add(2 * time.Second), "Say Hello", "* * * * *", SayHello, time.Now(), SUCCESS},
			{2, time.Now().Add(4 * time.Second), "Say Goodbye", "* * * * *", SayGoodbye, time.Now(), FAILURE},
		},
	}
}

type Scheduler struct {
	db    Database
	queue JobQueue
}

func NewScheduler(db Database, q JobQueue) *Scheduler {
	return &Scheduler{
		db:    db,
		queue: q,
	}
}

func (s *Scheduler) Run() {
	for {
		time.Sleep(time.Second * 1)
		log.Println("Polling...")
		s.Poll()
	}
}

func (s *Scheduler) Poll() {
	jobs, err := s.db.GetJobs()
	if err != nil {
		return
	}

	s.queue.PushJobs(jobs)
}

type Dispatcher struct {
	db    Database
	queue JobQueue
	ctx   context.Context
}

func (d *Dispatcher) Start() error {
	for {
		select {
		case job := <-d.queue.PullJob():
			go d.Worker(job, d.ctx)
		}
	}
}

// TODO implement retries and timeouts
func (d *Dispatcher) Worker(job Job, ctx context.Context) {
	if err := d.db.UpdateStatus(job.Id, INPROGRESS); err != nil {
		log.Println(err)
		return
	}

	if err := job.Handler(); err != nil {
		log.Println("Job Failed")
		d.db.UpdateStatus(job.Id, FAILURE)
		return
	}

	log.Println("Job success")
	d.db.UpdateStatus(job.Id, SUCCESS)
}

func NewDispatcher(db Database, q JobQueue) *Dispatcher {
	return &Dispatcher{
		db:    db,
		queue: q,
		ctx:   context.Background(),
	}
}

func main() {
	db := NewJobDB()
	scheduler := NewScheduler(db, NewChanQueue())
	go scheduler.Run()

	dispatcher := NewDispatcher(db, scheduler.queue)
	go dispatcher.Start()

	select {}
}
