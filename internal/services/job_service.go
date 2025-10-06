package services

import (
	"context"

	"github.com/Craig-Turley/task-scheduler.git/internal/repos"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/job"
	"github.com/Craig-Turley/task-scheduler.git/pkg/idgen"
	"github.com/bwmarrin/snowflake"
)

type JobService interface {
	// pass just the job the scheduler will take care of finding its next occurence
	CreateJob(ctx context.Context, job *job.Job) (*job.Job, error)
	GetJob(ctx context.Context, id snowflake.ID) (*job.Job, error)
}

type jobService struct {
	repo repos.JobRepo
}

// jobService implements the JobService interface
// can pass it any repo that fits the JobRepo interface
func NewJobService(repo repos.JobRepo) *jobService {
	return &jobService{repo: repo}
}

// do this in the orchestrator now
func (s *jobService) CreateJob(ctx context.Context, job *job.Job) (*job.Job, error) {
	if err := job.Validate(); err != nil {
		return nil, err
	}

	job.Id = idgen.NewId()
	return s.repo.CreateJob(ctx, job)
}

func (s *jobService) GetJob(ctx context.Context, id snowflake.ID) (*job.Job, error) {
	return s.repo.GetJob(ctx, id)
}
