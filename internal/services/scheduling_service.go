package services

import (
	"time"

	"github.com/Craig-Turley/task-scheduler.git/internal/repos"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/job"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/schedule"
	"github.com/Craig-Turley/task-scheduler.git/pkg/idgen"
	"github.com/Craig-Turley/task-scheduler.git/pkg/utils"
	"github.com/bwmarrin/snowflake"
)

type SchedulingService interface {
	ScheduleJob(jobId snowflake.ID, iso string) error
	GetJobsDueBefore(iso string) ([]*job.JobEvent, error)
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

// TODO fix the scheduling from cron to iso 
func (s *schedulingService) ScheduleJob(jobId snowflake.ID, iso string) error {
	t, err := time.Parse(time.RFC3339, iso); 
	if err != nil {
		return utils.NewError("Error parsing iso string %s", iso) 
	}

	// give it a buffer of an hour since the iso will always be before the current time
	if t.Before(time.Now().Add(time.Hour * -1).UTC()) {
		return utils.NewError("Error attempting to schedule a job in the past")
	}
		
	data := schedule.NewScheduleData(idgen.NewId(), jobId, iso, schedule.StatusScheduled)

	return s.repo.ScheduleEvent(data)
}

// TODO test this
func (s *schedulingService) GetJobsDueBefore(iso string) ([]*job.JobEvent, error) {
	if _, err := time.Parse(utils.TIME_FORMAT, iso); err != nil {
		return nil, err
	}

	return s.repo.GetJobsDueBefore(iso)
}

func (s *schedulingService) UpdateJobStatus(schedulingId snowflake.ID, status schedule.Status) error {
	return s.repo.UpdateJobStatus(schedulingId, status)
}
