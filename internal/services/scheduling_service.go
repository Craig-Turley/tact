package services

import (
	"time"

	"github.com/Craig-Turley/task-scheduler.git/internal/repos"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/job"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/schedule"
	"github.com/Craig-Turley/task-scheduler.git/pkg/idgen"
	"github.com/Craig-Turley/task-scheduler.git/pkg/utils"
	"github.com/bwmarrin/snowflake"
	"github.com/robfig/cron"
)

type SchedulingService interface {
	ScheduleJob(jobId snowflake.ID, cronStr string) error
	GetJobsDueBefore(timeStamp string) ([]*job.JobEvent, error)
	UpdateJobStatus(schedulingId snowflake.ID, newStatus schedule.Status) error
}

type schedulingService struct {
	repo repos.SchedulingRepo
}

func NewSchedulingService(repo repos.SchedulingRepo) *schedulingService {
	return &schedulingService{
		repo: repo,
	}
}

func (s *schedulingService) ScheduleJob(jobId snowflake.ID, cronStr string) error {
	sched, err := cron.Parse(cronStr)
	if err != nil {
		return err
	}

	runAt := sched.Next(time.Now().UTC()).Format(utils.TIME_FORMAT) // next run time after current time
	data := schedule.NewScheduleData(idgen.NewId(), jobId, runAt, schedule.StatusScheduled)

	return s.repo.ScheduleEvent(data)
}

func (s *schedulingService) GetJobsDueBefore(timeStamp string) ([]*job.JobEvent, error) {
	if _, err := time.Parse(utils.TIME_FORMAT, timeStamp); err != nil {
		return nil, err
	}

	return s.repo.GetJobsDueBefore(timeStamp)
}

func (s *schedulingService) UpdateJobStatus(schedulingId snowflake.ID, status schedule.Status) error {
	return s.repo.UpdateJobStatus(schedulingId, status)
}
