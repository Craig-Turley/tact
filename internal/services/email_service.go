package services

import (
	"github.com/Craig-Turley/task-scheduler.git/internal/repos"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/email"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/template"
	"github.com/bwmarrin/snowflake"
	_ "github.com/mattn/go-sqlite3"
)

type EmailService interface {
	CreateEmailJob(data *email.EmailData) error
	GetEmailJobData(jobId snowflake.ID) (*email.EmailData, error)
	GetEmailList(jobId snowflake.ID) ([]email.Email, error)
	GetTemplate(jobId snowflake.ID) (template.Template, error)
}

type emailService struct {
	repo          repos.EmailRepo
	templateStore repos.TemplateStore
}

func NewEmailService(repo repos.EmailRepo, store repos.TemplateStore) *emailService {
	return &emailService{
		repo:          repo,
		templateStore: store,
	}
}

func (s *emailService) CreateEmailJob(data *email.EmailData) error {
	return s.repo.SaveEmailData(data)
}

func (s *emailService) GetEmailJobData(jobId snowflake.ID) (*email.EmailData, error) {
	return s.repo.GetEmailData(jobId)
}

func (s *emailService) GetEmailList(jobId snowflake.ID) ([]email.Email, error) {
	return s.repo.GetEmailList(jobId)
}

func (s *emailService) GetTemplate(jobId snowflake.ID) (template.Template, error) {
	return s.templateStore.GetTemplate(jobId)
}
