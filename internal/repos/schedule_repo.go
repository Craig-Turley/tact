package repos

import (
	"database/sql"
	"errors"

	"github.com/Craig-Turley/task-scheduler.git/pkg/common/job"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/schedule"
	"github.com/bwmarrin/snowflake"
	_ "github.com/mattn/go-sqlite3"
)

type SchedulingRepo interface {
	ScheduleEvent(event *schedule.ScheduleData) error
	GetJobsDueBefore(timeStamp string) ([]*job.JobEvent, error)
	UpdateJobStatus(jobId snowflake.ID, status schedule.Status) error
}

type SqliteSchedulingRepo struct {
	store *sql.DB
}

func NewSqliteSchedulingRepo(db *sql.DB) *SqliteSchedulingRepo {
	return &SqliteSchedulingRepo{
		store: db,
	}
}

func (s SqliteSchedulingRepo) ScheduleEvent(event *schedule.ScheduleData) error {
	query := "INSERT INTO scheduling (id, job_id, run_at, status) VALUES (?, ?, ?, ?)"
	_, err := s.store.Exec(query, event.Id, event.JobId, event.RunAt, event.Status)
	if err != nil {
		return errors.New("Error inserting into table")
	}

	return nil
}

func (s SqliteSchedulingRepo) GetJobsDueBefore(timeStamp string) ([]*job.JobEvent, error) {
	query := "SELECT j.id, j.name, j.cron, j.retry_limit, j.type, s.id AS schedule_id FROM jobs j JOIN scheduling s ON s.job_id=j.id WHERE s.run_at < ? AND s.status = ?"
	rows, err := s.store.Query(query, timeStamp, schedule.StatusScheduled)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*job.JobEvent

	for rows.Next() {
		var entry job.JobEvent
		if err := rows.Scan(&entry.Id, &entry.Name, &entry.Cron, &entry.RetryLimit, &entry.Type, &entry.ScheduleId); err != nil {
			return nil, err
		}

		entries = append(entries, &entry)
	}

	return entries, nil
}

func (s SqliteSchedulingRepo) UpdateJobStatus(id snowflake.ID, status schedule.Status) error {
	query := "UPDATE scheduling SET status = ? WHERE id = ?"
	_, err := s.store.Exec(query, status, id)

	// TODO handle the case where multiple rows are updated
	// should only be one
	//

	// if res.RowsAffected() != 1 {
	//
	//  }
	return err
}
