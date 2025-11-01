package repos

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/Craig-Turley/task-scheduler.git/pkg/common/email"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/template"
	"github.com/Craig-Turley/task-scheduler.git/pkg/idgen"
	"github.com/Craig-Turley/task-scheduler.git/pkg/utils"
	"github.com/bwmarrin/snowflake"
	_ "github.com/mattn/go-sqlite3"
)

type EmailRepo interface {
	SaveEmailData(ctx context.Context, emailData *email.EmailData) error
	GetEmailData(ctx context.Context, jobId snowflake.ID) (*email.EmailData, error)
	CreateEmailList(ctx context.Context, listData *email.EmailListData) (snowflake.ID, error)
	DeleteEmailList(ctx context.Context, listId snowflake.ID) error
	GetEmailLists(ctx context.Context, userId string) ([]*email.EmailListData, error)
	GetEmailListSubscribers(ctx context.Context, listId snowflake.ID) ([]*email.SubscriberInformation, error)
	AddToEmailList(ctx context.Context, listId snowflake.ID, subs []*email.SubscriberInformation) error
	RemoveFromList(ctx context.Context, subId snowflake.ID) error
	GetEmailListData(ctx context.Context, listId snowflake.ID) (*email.EmailListData, error)
}

type SqliteEmailRepo struct {
	store         *sql.DB
	templateStore TemplateStore
}

func NewSqliteEmailRepo(db *sql.DB) *SqliteEmailRepo {
	return &SqliteEmailRepo{
		store:         db,
		templateStore: NewLocalTemplateStore(os.Getenv("TEMPLATE_DIR")),
	}
}

func (s *SqliteEmailRepo) SaveEmailData(ctx context.Context, data *email.EmailData) error {
	query := "INSERT INTO email_job_data (job_id, list_id, subject) VALUES (?, ?, ?)"
	_, err := s.store.ExecContext(ctx, query, data.JobId, data.ListId, data.Subject)
	return err
}

func (s *SqliteEmailRepo) GetEmailData(ctx context.Context, jobId snowflake.ID) (*email.EmailData, error) {
	var emailData email.EmailData
	row := s.store.QueryRowContext(ctx, "SELECT job_id, list_id, subject FROM email_job_data WHERE job_id = ?", jobId)
	if row.Err() != nil {
		return nil, row.Err()
	}

	err := row.Scan(&emailData.JobId, &emailData.ListId, &emailData.Subject)
	if err != nil {
		return nil, err
	}

	return &emailData, err
}

// keeping this here just incase i ever decide to go back to the email_addresses, email_lists, email_subscriptions db model
// query := `
//    SELECT ea.email
//    FROM email_addresses AS ea
//    JOIN subscriptions AS s ON s.email_address_id = ea.id
//    WHERE s.email_list_id = ?;
//  `

func (s *SqliteEmailRepo) CreateEmailList(ctx context.Context, data *email.EmailListData) (snowflake.ID, error) {
	if utils.ValidEmailListName(data.Name) == false {
		return -1, utils.NewError("Error: Invalid list name")
	}

	data.ListId = idgen.NewId()

	query := `
		INSERT INTO email_lists (id, name, user_id)
		VALUES (?, ?, ?)
	`
	_, err := s.store.ExecContext(ctx, query, data.ListId, data.Name, data.UserId)
	return data.ListId, err
}

func (s *SqliteEmailRepo) DeleteEmailList(ctx context.Context, listId snowflake.ID) error {
	query := `
		DELETE FROM email_lists
		WHERE id = ?
	`
	_, err := s.store.ExecContext(ctx, query, listId)
	return err
}

func (s *SqliteEmailRepo) GetEmailLists(ctx context.Context, userId string) ([]*email.EmailListData, error) {
	query := `
		SELECT 
				l.id,
				l.name,
				l.user_id,
				COUNT(s.id) AS subscriber_count
		FROM email_lists l
		LEFT JOIN subscribers s 
				ON s.list_id = l.id 
				AND s.is_subscribed = TRUE
		WHERE l.user_id = ?
		GROUP BY l.id, l.name, l.user_id;
	`

	result, err := s.store.QueryContext(ctx, query, userId)
	if err != nil {
		return nil, err
	}

	var lists []*email.EmailListData
	for result.Next() {
		var list email.EmailListData
		result.Scan(&list.ListId, &list.Name, &list.UserId, &list.SubscriberCount)
		lists = append(lists, &list)
	}

	return lists, nil
}

func (s *SqliteEmailRepo) GetEmailListSubscribers(ctx context.Context, listId snowflake.ID) ([]*email.SubscriberInformation, error) {
	query := `
		SELECT id, email, first_name, last_name, list_id, is_subscribed 
		FROM subscribers AS s
		WHERE s.list_id = ? 
		AND is_subscribed = 1
	`

	rows, err := s.store.QueryContext(ctx, query, listId)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var subscribers []*email.SubscriberInformation
	for rows.Next() {
		var sub email.SubscriberInformation
		if err := rows.Scan(&sub.Id, &sub.Email, &sub.FirstName, &sub.LastName, &sub.ListId, &sub.IsSubscribed); err != nil {
			return nil, fmt.Errorf("row scan failed: %w", err)
		}
		subscribers = append(subscribers, &sub)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return subscribers, nil
}

func (s *SqliteEmailRepo) GetEmailListData(ctx context.Context, listId snowflake.ID) (*email.EmailListData, error) {
	query := `
		SELECT id, name FROM email_lists AS el
		WHERE el.id = ?
	`

	row := s.store.QueryRowContext(ctx, query, listId)
	if row.Err() != nil {
		return nil, row.Err()
	}

	var listData email.EmailListData
	if err := row.Scan(&listData.ListId, &listData.Name); err != nil {
		return nil, err
	}

	return &listData, nil
}

func (s *SqliteEmailRepo) AddToEmailList(ctx context.Context, listId snowflake.ID, subs []*email.SubscriberInformation) error {
	query := `
		INSERT INTO subscribers
		(id, email, first_name, last_name, list_id, is_subscribed)
		VALUES
		(?, ?, ?, ?, ?, ?)
	`

	failed := []*email.SubscriberInformation{}
	var err error
	for _, sub := range subs {
		sub.Id = idgen.NewId()
		sub.ListId = listId
		_, err = s.store.ExecContext(ctx, query, sub.Id, sub.Email, sub.FirstName, sub.LastName, sub.ListId, sub.IsSubscribed)
		if err != nil {
			failed = append(failed, sub)
		}
	}

	if err != nil {
		err = utils.NewError("failed to insert %d subscribers for listId %d", len(subs), listId)
	}

	return err
}

func (s *SqliteEmailRepo) RemoveFromList(ctx context.Context, subId snowflake.ID) error {
	query := `
		DELETE FROM subscribers
		WHERE id = ?
	`

	if _, err := s.store.ExecContext(ctx, query, subId); err != nil {
		return err
	}

	return nil
}

// TODO fix this
func (s *SqliteEmailRepo) GetTemplate(ctx context.Context, jobId snowflake.ID) (template.Template, error) {
	// s.mu.Lock()
	// defer s.mu.Unlock()
	//
	// row := s.store.QueryRow("SELECT path FROM email_template WHERE schedule_id=?", scheduleId)
	//
	// var path string
	// if err := row.Scan(&path); err != nil {
	// 	return "", err
	// }
	//
	// template, err := os.ReadFile("file.txt")
	// if err != nil {
	// 	return "", err
	// }
	//
	// return string(template), nil
	return s.templateStore.GetTemplate(ctx, jobId)
}
