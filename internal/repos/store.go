package repos

import "database/sql"

type Storage struct {
	Jobs     JobRepo
	Email    EmailRepo
	Template TemplateStore
}

func NewSqliteStore(db *sql.DB, templateStorePath string) *Storage {
	return &Storage{
		Jobs:     NewSqliteJobRepo(db),
		Email:    NewSqliteEmailRepo(db),
		Template: NewLocalTemplateStore(templateStorePath),
	}
}
