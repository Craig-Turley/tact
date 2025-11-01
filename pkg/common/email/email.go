package email

import (
	"github.com/bwmarrin/snowflake"
)

type Email string

type EmailData struct {
	JobId   snowflake.ID `json:"job_id"`
	ListId  snowflake.ID `json:"list_id"`
	Subject string       `json:"subject"`
}

func NewEmailData(jobId, listId snowflake.ID, subject string) *EmailData {
	return &EmailData{
		JobId:   jobId,
		ListId:  listId,
		Subject: subject,
	}
}

type SubscriberInformation struct {
	Id           snowflake.ID
	FirstName    string
	LastName     string
	Email        Email
	ListId       snowflake.ID
	IsSubscribed bool
}

func NewSubscriberInformation(
	id snowflake.ID,
	firstName string,
	lastName string,
	email Email,
	listId snowflake.ID,
	isSubscribed bool,
) *SubscriberInformation {
	return &SubscriberInformation{
		Id:           id,
		FirstName:    firstName,
		LastName:     lastName,
		Email:        email,
		ListId:       listId,
		IsSubscribed: isSubscribed,
	}
}

type EmailListData struct {
	ListId          snowflake.ID `json:"list_id"`
	Name            string       `json:"name"`
	UserId          string       `json:"user_id"` // provided by goth
	SubscriberCount int          `json:"subscriber_count"`
}

func NewEmailListData(listId snowflake.ID, name string, userId string) *EmailListData {
	return &EmailListData{
		ListId: listId,
		Name:   name,
		UserId: userId,
	}
}
