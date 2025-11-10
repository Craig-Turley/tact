package repos

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Craig-Turley/task-scheduler.git/pkg/common/job"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/schedule"
	"github.com/Craig-Turley/task-scheduler.git/pkg/idgen"
	"github.com/bwmarrin/snowflake"
	_ "github.com/mattn/go-sqlite3"
)

type SchedulingRepo interface {
	ScheduleEvent(ctx context.Context, event *schedule.ScheduleData) (*schedule.ScheduleData, error)
	DeleteEvent(ctx context.Context, id snowflake.ID) error
	GetJobsDueBefore(ctx context.Context, iso string) ([]*job.JobEvent, error)
	UpdateJobStatus(ctx context.Context, jobId snowflake.ID, status schedule.Status) error
}

type SqliteSchedulingRepo struct {
	store *sql.DB
}

func NewSqliteSchedulingRepo(db *sql.DB) *SqliteSchedulingRepo {
	return &SqliteSchedulingRepo{
		store: db,
	}
}

func (s SqliteSchedulingRepo) ScheduleEvent(ctx context.Context, event *schedule.ScheduleData) (*schedule.ScheduleData, error) {
	event.Id = idgen.NewId()
	query := "INSERT INTO scheduling (id, job_id, run_at, status, user_id) VALUES (?, ?, ?, ?, ?)"
	_, err := s.store.ExecContext(ctx, query, event.Id, event.JobId, event.RunAt, event.Status, event.UserId)
	if err != nil {
		return nil, errors.New("Error inserting into table")
	}

	return event, nil
}

func (s SqliteSchedulingRepo) DeleteEvent(ctx context.Context, id snowflake.ID) error {
	query := "DELETE FROM scheduling s WHERE s.id = ?"
	_, err := s.store.Exec(query, id)
	if err != nil {
		return errors.New("Error inserting into table")
	}

	return nil
}

func (s SqliteSchedulingRepo) GetJobsDueBefore(ctx context.Context, iso string) ([]*job.JobEvent, error) {
	query := "SELECT j.id, j.name, j.retry_limit, j.type, s.id AS schedule_id FROM jobs j JOIN scheduling s ON s.job_id=j.id WHERE s.run_at < ? AND s.status = ?"
	rows, err := s.store.Query(query, iso, schedule.StatusScheduled)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*job.JobEvent

	for rows.Next() {
		var entry job.JobEvent
		if err := rows.Scan(&entry.Id, &entry.Name, &entry.RetryLimit, &entry.Type, &entry.ScheduleId); err != nil {
			return nil, err
		}

		entries = append(entries, &entry)
	}

	return entries, nil
}

func (s SqliteSchedulingRepo) UpdateJobStatus(ctx context.Context, jobId snowflake.ID, status schedule.Status) error {
	query := "UPDATE scheduling SET status = ? WHERE id = ?"
	_, err := s.store.ExecContext(ctx, query, status, jobId)

	// TODO handle the case where multiple rows are updated
	// should only be one
	//

	// if res.RowsAffected() != 1 {
	//
	//  }
	return err
}
