package services

import (
	"github.com/Craig-Turley/task-scheduler.git/internal/repos"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/job"
	"github.com/Craig-Turley/task-scheduler.git/pkg/idgen"
	"github.com/bwmarrin/snowflake"
	"github.com/robfig/cron"
)

type JobService interface {
	// pass just the job the scheduler will take care of finding its next occurence
	CreateJob(job *job.Job) error
	GetJob(id snowflake.ID) (*job.Job, error)
}

type jobService struct {
	repo repos.JobRepo
}

// jobService implements the JobService interface
// can pass it any repo that fits the JobRepo interface
func NewJobService(repo repos.JobRepo) *jobService {
	return &jobService{repo: repo}
}

func (s *jobService) CreateJob(job *job.Job) error {
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

func (s *jobService) GetJob(id snowflake.ID) (*job.Job, error) {
	return s.repo.GetJob(id)
}
