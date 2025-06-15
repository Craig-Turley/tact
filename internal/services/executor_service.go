package services

import (
	"log"
	"time"

	"github.com/Craig-Turley/task-scheduler.git/pkg/utils"
)

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
	workerSrvc     WorkerService
}

func NewExecutorService(
	tick time.Duration,
	jobSrvc JobService,
	emailSrvc EmailService,
	schedulingSrvc SchedulingService,
	workerSrvc WorkerService,
) *executorService {
	return &executorService{
		Tick:           tick,
		jobSrvc:        jobSrvc,
		emailSrvc:      emailSrvc,
		schedulingSrvc: schedulingSrvc,
		workerSrvc:     workerSrvc,
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
		e.workerSrvc.Enque(event)
	}
}
