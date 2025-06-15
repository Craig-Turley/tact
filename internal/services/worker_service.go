package services

import (
	"log"

	"github.com/Craig-Turley/task-scheduler.git/pkg/common/job"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/schedule"
	"github.com/Craig-Turley/task-scheduler.git/pkg/utils"
)

type Runner func(e *job.Job) error

type WorkerService interface {
	Enque(e *job.JobEvent)
}

type LocalWorkerService struct {
	schedulingSrvc SchedulingService
	emailSrvc      EmailService
}

func NewLocalWorkerService(schedulingSrvc SchedulingService, emailSrvc EmailService) *LocalWorkerService {
	return &LocalWorkerService{
		schedulingSrvc: schedulingSrvc,
		emailSrvc:      emailSrvc,
	}
}

func (w *LocalWorkerService) getRunner(t job.JobType) (Runner, error) {
	switch t {
	case job.TypeEmail:
		return w.runEmail, nil
	}

	return nil, utils.NewError(utils.ERROR_JOB_RUNNER_NOT_FOUND)
}

func (w *LocalWorkerService) Enque(e *job.JobEvent) {
	go w.worker(e)
}

func (w *LocalWorkerService) worker(e *job.JobEvent) {
	// TODO handle error
	if !e.Type.Valid() {
		log.Println(utils.ERROR_JOB_TYPE_INVALID)
		return
	}

	// get runner
	runner, err := w.getRunner(e.Type)
	if err != nil {
		log.Println(err)
		return
	}

	// run job
	retry := e.RetryLimit
	for {
		err = runner(&e.Job)
		if err == nil || retry == 0 {
			break
		}
		log.Println(err)
		retry--
	}

	// TODO fix the below error handling (Update returns an error)
	if err != nil {
		log.Printf("Failed on ScheduleId %s: %s", e.ScheduleId, err)
		w.schedulingSrvc.UpdateJobStatus(e.ScheduleId, schedule.StatusFailed)
		return
	}

	// TODO return error
	// update status
	if err = w.schedulingSrvc.UpdateJobStatus(e.ScheduleId, schedule.StatusSuccess); err != nil {
		log.Printf("Failed on ScheduleId %s: %s", e.ScheduleId, err)
		return
	}

	if err = w.schedulingSrvc.ScheduleJob(e.Id, e.Cron); err != nil {
		log.Printf("Failed on ScheduleId %s: %s", e.ScheduleId, err)
		return
	}

	log.Printf("ScheduleId %d success", e.ScheduleId)
}

func (w *LocalWorkerService) runEmail(e *job.Job) error {
	data, err := w.emailSrvc.GetEmailJobData(e.Id)
	if err != nil {
		return err
	}

	list, err := w.emailSrvc.GetEmailList(data.ListId)
	if err != nil {
		return err
	}

	template, err := w.emailSrvc.GetTemplate(data.ListId)
	if err != nil {
		return err
	}

	// TODO remove this
	template.Sanitize()

	for _, e := range list {
		log.Printf("sending email to %s", e)
	}

	return nil
}
