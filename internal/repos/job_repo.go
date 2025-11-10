package repos

import (
	"context"
	"database/sql"
	"sync"

	"github.com/Craig-Turley/task-scheduler.git/pkg/common/job"
	"github.com/Craig-Turley/task-scheduler.git/pkg/idgen"
	"github.com/bwmarrin/snowflake"
	_ "github.com/mattn/go-sqlite3"
)

type GetUserJobOpts struct {
	JobType *job.JobType
}

type JobRepo interface {
	CreateJob(ctx context.Context, job *job.Job) (*job.Job, error)
	GetJob(ctx context.Context, id snowflake.ID) (*job.Job, error)
	GetUserJobs(ctx context.Context, userId string, opts *GetUserJobOpts) ([]*job.Job, error)
	DeleteJob(ctx context.Context, id snowflake.ID) error
}

type SqliteJobRepo struct {
	store   *sql.DB
	mu      sync.Mutex
	cdcChan chan *job.Job // not sure about this. supposed to mimic cdc but not sure if I want/need it for my 0 users
}

func NewSqliteJobRepo(db *sql.DB) *SqliteJobRepo {
	return &SqliteJobRepo{
		store:   db,
		mu:      sync.Mutex{},
		cdcChan: make(chan *job.Job),
	}
}

// CreateJob sets the id of the job passed in from autoincrementing table and returns an erro
func (s *SqliteJobRepo) CreateJob(ctx context.Context, job *job.Job) (*job.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job.Id = idgen.NewId()
	query := "INSERT INTO jobs (id, user_id, name, retry_limit, type) VALUES (?, ?, ?, ?, ?)"
	_, err := s.store.ExecContext(ctx, query, job.Id, job.UserId, job.Name, job.RetryLimit, job.Type)
	if err != nil {
		return nil, err
	}

	return job, nil
}

// ahh yes, when this has a lot of history than say goodbye to your memory
func (s *SqliteJobRepo) GetJob(ctx context.Context, id snowflake.ID) (*job.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	row := s.store.QueryRowContext(ctx, "SELECT * FROM jobs WHERE id=?", id)
	if err := row.Err(); err != nil {
		return nil, err
	}

	var job job.Job
	if err := row.Scan(&job.Id, &job.Name, &job.RetryLimit, &job.Type); err != nil {
		return nil, err
	}

	return &job, nil
}

func (s *SqliteJobRepo) GetUserJobs(ctx context.Context, userId string, opts *GetUserJobOpts) ([]*job.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := "SELECT id, name, retry_limit, type, user_id FROM jobs WHERE user_id = ?"
	args := []any{userId}

	if opts != nil {
		if opts.JobType != nil {
			query += " AND type = ?"
			args = append(args, uint8(*opts.JobType))
		}
	}

	rows, err := s.store.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*job.Job
	for rows.Next() {
		var j job.Job
		if err := rows.Scan(&j.Id, &j.Name, &j.RetryLimit, &j.Type, &j.UserId); err != nil {
			return nil, err
		}
		jobs = append(jobs, &j)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return jobs, nil
}

func (s *SqliteJobRepo) DeleteJob(ctx context.Context, id snowflake.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := "DELETE FROM jobs WHERE id = "
	_, err := s.store.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	return nil
}
