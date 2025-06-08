package email

import "github.com/bwmarrin/snowflake"

type Email string

type EmailData struct {
	JobId  snowflake.ID `json:"job_id"`
	ListId snowflake.ID `json:"list_id"`
}

func NewEmailData(jobId, listId snowflake.ID) *EmailData {
	return &EmailData{
		JobId:  jobId,
		ListId: listId,
	}
}
