package main

import (
	"log"
	"time"

	// _ "github.com/mattn/go-sqlite3"
	"github.com/robfig/cron"
)

type CDCQueue interface {
	Listen() <-chan *Job
}

type Job struct {
	Id   int
	Name string
	Cron string
}

func NewJob(name, cron string) *Job {
	return &Job{
		Name: name,
		Cron: cron,
	}
}

type Database interface {
	Create(job *Job) error
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

func (db *DBWithCDC) Create(job *Job) error {
	job.Id = db.idInc
	db.idInc += 1

	db.store[job.Id] = job
	db.cdcChan <- job

	return nil
}

func (db *DBWithCDC) Listen() <-chan *Job {
	return db.cdcChan
}

type QueService struct {
	db Database
}

func NewQueService(db Database) *QueService {
	return &QueService{
		db: db,
	}
}

func (q *QueService) Enqueue(job *Job) error {
	if err := q.db.Create(job); err != nil {
		return err
	}

	return nil
}

type Scheduler struct {
	cdcQueue   CDCQueue
	scheduleDb Database
}

func NewScheduler(que CDCQueue, scheduleDb Database) *Scheduler {
	return &Scheduler{
		cdcQueue:   que,
		scheduleDb: scheduleDb,
	}
}

func (s *Scheduler) Run() {
	for {
		select {
		case job := <-s.cdcQueue.Listen():
			s.Schedule(job)
		}
	}
}

func (s *Scheduler) Schedule(job *Job) {
	// TODO handler error
	if err := s.scheduleDb.Create(job); err != nil {
		panic(err)
	}
}

type ScheduleDBEntry struct {
	*Job
	runAt string
}

type ScheduleDB struct {
	store map[int]*ScheduleDBEntry
}

func NewScheduleDB() *ScheduleDB {
	return &ScheduleDB{
		store: make(map[int]*ScheduleDBEntry),
	}
}

func (s *ScheduleDB) Create(job *Job) error {
	schedule, err := cron.Parse(job.Cron)
	if err != nil {
		return (err)
	}

	runAt := schedule.Next(time.Now())
	t := runAt.UTC().Format("2006-01-02T15:04:05.000Z")
	s.store[job.Id] = &ScheduleDBEntry{job, t}

	for _, e := range s.store {
		log.Println(e.Name, e.runAt)
	}

	return nil
}

func main() {
	jobsDb := NewDBWithCDC()
	queService := NewQueService(jobsDb)

	scheduleDB := NewScheduleDB()
	scheduler := NewScheduler(jobsDb, scheduleDB)
	go scheduler.Run()

	queService.Enqueue(NewJob("Say Hello", "* * * * * *"))
	queService.Enqueue(NewJob("Say Goodbye", "* * * * * *"))

	select {}
}
