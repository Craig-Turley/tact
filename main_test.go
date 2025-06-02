package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
)

// type JobRepo interface {
// 	CreateJob(job *Job) error
// 	GetJobs() ([]*Job, error)
// 	SaveEmailData(data EmailData) error
// 	GetEmailData(jobId int) (EmailData, error)
// }

const RETURN_ID = 10

var (
	TEST_URL         = "http://localhost:8080"
	TEST_SERVER_PORT = ":8080"
)

type TestJobRepo struct {
	returnId int
	idCache  []int
}

func NewTestJobRepo(returnId int) *TestJobRepo {
	return &TestJobRepo{
		returnId: returnId,
	}
}

func (t *TestJobRepo) CreateJob(job *Job) error {
	job.Id = t.returnId
	return nil
}

func (t *TestJobRepo) GetJobs() ([]*Job, error) {
	return nil, nil
}

func (t *TestJobRepo) SaveEmailData(data EmailData) error {
	t.idCache = append(t.idCache, data.JobId)
	return nil
}

func (t *TestJobRepo) GetEmailData(jobId int) (EmailData, error) {
	return EmailData{}, nil
}

func TestCronMiddleware(t *testing.T) {
	testJobRepo := NewTestJobRepo(RETURN_ID)
	server := NewServer(testJobRepo, fmt.Sprintf(TEST_SERVER_PORT))

	ready := make(chan struct{})
	go func() {
		close(ready)
		t.Log(server.Start())
	}()
	<-ready

	jsonData := []byte(`{
    "name": "weekly_email",
    "cron": "* * * * * *",
    "retry_limit": 3,
    "job_type": 2,
    "list_id": 2
  }`)

	if !json.Valid(jsonData) {
		t.Error("Testing with invalid json data")
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/cron/email", TEST_URL), bytes.NewReader(jsonData))
	if err != nil {
		t.Error(err)
	}

	req.Header.Set("Content-Type", "applcation/json; charset=UTF-8")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Error(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Error(body)
	}
	// t.Log(string(body))

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Failing status code %d recieved. Expected %d", resp.StatusCode, http.StatusCreated)
	}

	if len(testJobRepo.idCache) == 0 {
		t.Errorf("No id's cached")
	}

	for _, id := range testJobRepo.idCache {
		if id != testJobRepo.returnId {
			t.Errorf("Invalid id returned. Got %d wanted %d", id, testJobRepo.returnId)
		}
	}
}
