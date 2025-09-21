package repos_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Craig-Turley/task-scheduler.git/internal/repos"
	"github.com/Craig-Turley/task-scheduler.git/pkg/common/job"
	"github.com/Craig-Turley/task-scheduler.git/pkg/idgen"
)


func TestJobRepo(t *testing.T) {
	jobRepo := repos.NewSqliteJobRepo(sqlite3db)
	
	tests := []*job.Job{
		{Id: idgen.NewId(), Name: "Test 1", RetryLimit: 3, Type: job.TypeEmail},
		{Id: idgen.NewId(), Name: "Welcome Email", RetryLimit: 5, Type: job.TypeEmail},
		{Id: idgen.NewId(), Name: "Generic Email", RetryLimit: 3, Type: job.TypeEmail},
		{Id: idgen.NewId(), Name: "Weekly Newsletter Email", RetryLimit: 2, Type: job.TypeEmail},
		{Id: idgen.NewId(), Name: "Promotional Campaign Email", RetryLimit: 4, Type: job.TypeEmail},
		{Id: idgen.NewId(), Name: "Account Verification Email", RetryLimit: 3, Type: job.TypeEmail},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	t.Cleanup(cancel)
	
	for i, tc := range tests {
		if err := jobRepo.CreateJob(ctx, tc); err != nil {
			t.Fatalf("create job %d (%s): %v", i, tc.Name, err)
		}
	}

	for i, tc := range tests {
		t.Run(fmt.Sprintf("%02d/%s", i, tc.Name), func(t *testing.T) {
			got, err := jobRepo.GetJob(ctx, tc.Id)
			if err != nil {
				t.Fatalf("get job %d (%s): %v", i, tc.Name, err)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("validate returned job %d (%s): %v", i, tc.Name, err)
			}

			if got.Id != tc.Id {
				t.Fatalf("Id mismatch: got %q want %q", got.Id, tc.Id)
			}
			if got.Name != tc.Name {
				t.Fatalf("Name mismatch: got %q want %q", got.Name, tc.Name)
			}
			if got.RetryLimit != tc.RetryLimit {
				t.Fatalf("RetryLimit mismatch: got %d want %d", got.RetryLimit, tc.RetryLimit)
			}
			if got.Type != tc.Type {
				t.Fatalf("Type mismatch: got %v want %v", got.Type, tc.Type)
			}
		})
	}

	t.Cleanup(func() {
		for _, tc := range tests {
			if _, err := sqlite3db.ExecContext(ctx, `DELETE FROM jobs WHERE id = ?`, tc.Id); err != nil {
				t.Errorf("cleanup delete %q: %v", tc.Id, err)
			}
		}
	})
}
