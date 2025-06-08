package repos

import (
	"database/sql"
	"fmt"
	"os"
	"sync"

	"github.com/Craig-Turley/task-scheduler.git/pkg/common/email"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/template"
	"github.com/bwmarrin/snowflake"
	_ "github.com/mattn/go-sqlite3"
)

// Using this as a type alias for an email string
// possibly might move this later

// TODO add context support
type EmailRepo interface {
	SaveEmailData(data *email.EmailData) error
	GetEmailData(jobId snowflake.ID) (*email.EmailData, error)
	GetEmailList(listId snowflake.ID) ([]email.Email, error)
}

// TODO add context support
type SqliteEmailRepo struct {
	store         *sql.DB
	templateStore TemplateStore
	mu            sync.Mutex
}

func NewSqliteEmailRepo(db *sql.DB) *SqliteEmailRepo {
	return &SqliteEmailRepo{
		store:         db,
		templateStore: NewLocalTemplateStore(os.Getenv("TEMPLATE_DIR")),
		mu:            sync.Mutex{},
	}
}

func (s *SqliteEmailRepo) SaveEmailData(data *email.EmailData) error {
	query := "INSERT INTO email_job_data (job_id, list_id) VALUES (?, ?)"
	_, err := s.store.Exec(query, data.JobId, data.ListId)
	return err
}

func (s *SqliteEmailRepo) GetEmailData(jobId snowflake.ID) (*email.EmailData, error) {
	var emailData email.EmailData
	row := s.store.QueryRow("SELECT * FROM email_job_data WHERE job_id = ?", jobId)
	if row.Err() != nil {
		return nil, row.Err()
	}

	err := row.Scan(&emailData.JobId, &emailData.ListId)
	if err != nil {
		return nil, err
	}

	return &emailData, err
}

//
// func (s *SqliteEmailRepo) GetListId(jobId int) (int, error) {
// 	s.mu.Lock()
// 	defer s.mu.Unlock()
//
// 	query := `SELECT list_id FROM email_job_data WHERE job_id=?`
//
// 	row := s.store.QueryRow(query, jobId)
// 	var listId int
// 	if err := row.Scan(&listId); err != nil {
// 		return -1, err
// 	}
//
// 	return listId, nil
// }

func (s *SqliteEmailRepo) GetEmailList(listId snowflake.ID) ([]email.Email, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
    SELECT ea.email
    FROM email_addresses AS ea
    JOIN subscriptions AS s ON s.email_address_id = ea.id
    WHERE s.email_list_id = ?;
  `

	rows, err := s.store.Query(query, listId)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var emails []email.Email
	for rows.Next() {
		var e string
		if err = rows.Scan(&e); err != nil {
			return nil, fmt.Errorf("row scan failed: %w", err)
		}
		emails = append(emails, email.Email(e))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return emails, nil
}

// TODO fix this
func (s *SqliteEmailRepo) GetTemplate(jobId snowflake.ID) (template.Template, error) {
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
	return s.templateStore.GetTemplate(jobId)
}
