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
	ListId snowflake.ID `json:"list_id"`
	Name   string       `json:"name"`
}

func NewEmailListData(listId snowflake.ID, name string) *EmailListData {
	return &EmailListData{
		ListId: listId,
		Name:   name,
	}
}
