package job

import (
	"github.com/Craig-Turley/task-scheduler.git/pkg/utils"
	"github.com/bwmarrin/snowflake"
)

type JobType uint8

const (
	TypeStart JobType = iota
	// custom scripts
	TypeCustom
	// automated messaging
	TypeEmail
	TypeSlack
	TypeDiscord

	TypeEnd
)

func (t JobType) Valid() bool {
	return TypeStart < t && t < TypeEnd
}

func JobTypeToString(t JobType) string {
	switch t {
	case TypeCustom:
		return "Custom"
	case TypeEmail:
		return "Email"
	case TypeSlack:
		return "Slack"
	case TypeDiscord:
		return "Discord"
	}

	return "not found"
}

type Job struct {
	Id         snowflake.ID `json:"job_id"`
	Name       string       `json:"name"`
	Cron       string       `json:"cron"`
	RetryLimit int          `json:"retry_limit"`
	Type       JobType      `json:"job_type"`
}

func NewJob(name, cron string, retryLimit int, jobType JobType) *Job {
	return &Job{
		Name:       name,
		Cron:       cron,
		RetryLimit: retryLimit,
		Type:       jobType,
	}
}

// Validates the job fields provided
func (j *Job) Validate() error {
	if len(j.Name) == 0 {
		return utils.NewError(utils.ERROR_JOB_NAME_NOT_PROVIDED)
	}

	if ok := j.Type.Valid(); !ok {
		return utils.NewError(utils.ERROR_JOB_TYPE_INVALID)
	}

	if j.RetryLimit <= 0 {
		return utils.NewError(utils.ERROR_INVALID_RETRY_LIMIT)
	}

	return nil
}

type JobEvent struct {
	Job
	ScheduleId snowflake.ID
}
