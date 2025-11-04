package schedule

import "github.com/bwmarrin/snowflake"

type Status uint8

const (
	StatusStart Status = iota
	StatusScheduled
	StatusFailed
	StatusRunning
	StatusSuccess
	StatusEnd
)

func StatusToString(s Status) string {
	switch s {
	case StatusScheduled:
		return "Scheduled"
	case StatusFailed:
		return "Failed"
	case StatusRunning:
		return "Running"
	case StatusSuccess:
		return "Success"
	}

	return "Status Unkown"
}

type ScheduleData struct {
	Id     snowflake.ID
	JobId  snowflake.ID
	RunAt  string `json:"run_at"`
	Status Status
}

func NewScheduleData(id, jobId snowflake.ID, runAt string, status Status) *ScheduleData {
	return &ScheduleData{
		Id:     id,
		JobId:  jobId,
		RunAt:  runAt,
		Status: status,
	}
}
