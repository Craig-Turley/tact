package services

import (
	"context"

	"github.com/Craig-Turley/task-scheduler.git/internal/repos"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/email"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/template"
	"github.com/Craig-Turley/task-scheduler.git/pkg/idgen"
	"github.com/bwmarrin/snowflake"
	_ "github.com/mattn/go-sqlite3"
)

type EmailService interface {
	CreateEmailJob(ctx context.Context, data *email.EmailData) error
	GetEmailJobData(ctx context.Context, jobID snowflake.ID) (*email.EmailData, error)
	CreateEmailList(ctx context.Context, data *email.EmailListData) (snowflake.ID, error)
	GetEmailListSubscribers(ctx context.Context, jobID snowflake.ID) ([]*email.SubscriberInformation, error)
	GetEmailListData(ctx context.Context, listId snowflake.ID) (*email.EmailListData, error)
	AddToEmailList(ctx context.Context, listId snowflake.ID, subs []*email.SubscriberInformation) ([]*email.SubscriberInformation, error)
	GetTemplate(ctx context.Context, jobID snowflake.ID) (template.Template, error)
	SaveTemplate(ctx context.Context, jobID snowflake.ID, t template.Template) error
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

func (s *emailService) CreateEmailJob(ctx context.Context, data *email.EmailData) error {
	return s.repo.SaveEmailData(ctx, data)
}

func (s *emailService) GetEmailJobData(ctx context.Context, jobID snowflake.ID) (*email.EmailData, error) {
	return s.repo.GetEmailData(ctx, jobID)
}

func (s *emailService) CreateEmailList(ctx context.Context, data *email.EmailListData) (snowflake.ID, error) {
	data.ListId = idgen.NewId()
	return s.repo.CreateEmailList(ctx, data)
}

func (s *emailService) GetEmailListSubscribers(ctx context.Context, jobID snowflake.ID) ([]*email.SubscriberInformation, error) {
	return s.repo.GetEmailListSubscribers(ctx, jobID)
}

func (s *emailService) GetEmailListData(ctx context.Context, listId snowflake.ID) (*email.EmailListData, error) {
	return s.repo.GetEmailListData(ctx, listId)
}

func (s *emailService) AddToEmailList(ctx context.Context, listId snowflake.ID, subs []*email.SubscriberInformation) ([]*email.SubscriberInformation, error) {
	for _, sub := range subs {
		sub.Id = idgen.NewId()
		sub.ListId = listId
	}

	return s.repo.AddToEmailList(ctx, listId, subs)
}

func (s *emailService) GetTemplate(ctx context.Context, jobID snowflake.ID) (template.Template, error) {
	return s.templateStore.GetTemplate(ctx, jobID)
}

func (s *emailService) SaveTemplate(ctx context.Context, jobID snowflake.ID, templ template.Template) error {
	return s.templateStore.SaveTemplate(ctx, jobID, string(templ))
}
