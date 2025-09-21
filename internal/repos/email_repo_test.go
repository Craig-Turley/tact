package repos_test

import (
    "context"
    "fmt"
    "testing"
    "time"

    "github.com/Craig-Turley/task-scheduler.git/internal/repos"
    "github.com/Craig-Turley/task-scheduler.git/pkg/common/email"
    "github.com/Craig-Turley/task-scheduler.git/pkg/idgen"
)

const TESTS = 5
const SUBSPERTEST = 10

func TestEmailRepo(t *testing.T) {
	emailRepo := repos.NewSqliteEmailRepo(sqlite3db)

	type EmailRepoTestData struct {
			emailData     *email.EmailData
			emailListData *email.EmailListData
			subscribers   []*email.SubscriberInformation
	}

	tests := make([]EmailRepoTestData, 0, TESTS)
	for i := range TESTS {
		ed := email.NewEmailData(idgen.NewId(), idgen.NewId())
		eld := email.NewEmailListData(ed.ListId, fmt.Sprintf("List %d", i))
		subs := make([]*email.SubscriberInformation, 0, SUBSPERTEST)
		for j := range SUBSPERTEST {
			subs = append(subs, email.NewSubscriberInformation(
				idgen.NewId(), "first", "last",
				email.Email(fmt.Sprintf("test+%d@email.com", j)),
				ed.ListId, true,
			))
		}
		tests = append(tests, EmailRepoTestData{emailData: ed, emailListData: eld, subscribers: subs})
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = sqlite3db.ExecContext(ctx, "DELETE FROM email_job_data")
		_, _ = sqlite3db.ExecContext(ctx, "DELETE FROM email_lists")
		_, _ = sqlite3db.ExecContext(ctx, "DELETE FROM subscribers")
	})

	for i, tc := range tests {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := emailRepo.SaveEmailData(ctx, tc.emailData); err != nil {
				t.Fatalf("save email data %d: %v", i, err)
		}
	}

	for i, tc := range tests {
		i, tc := i, tc 
		t.Run(fmt.Sprintf("%02d/GetEmailData", i), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
			defer cancel()

			got, err := emailRepo.GetEmailData(ctx, tc.emailData.JobId)
			if err != nil {
					t.Fatalf("GetEmailData %d (job:%v): %v", i, tc.emailData.JobId, err)
			}

			if got.JobId != tc.emailData.JobId {
					t.Fatalf("JobId mismatch: got %v want %v", got.JobId, tc.emailData.JobId)
			}

			if got.ListId != tc.emailData.ListId {
					t.Fatalf("ListId mismatch: got %v want %v", got.ListId, tc.emailData.ListId)
			}
		})
	}

	for i, tc := range tests {
		ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
		defer cancel()
		if _, err := emailRepo.CreateEmailList(ctx, tc.emailListData); err != nil {
				t.Fatalf("create email list %d: %v", i, err)
		}
	}

	for i, tc := range tests {
		i, tc := i, tc
		t.Run(fmt.Sprintf("%02d/GetEmailListData", i), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
			defer cancel()

			got, err := emailRepo.GetEmailListData(ctx, tc.emailListData.ListId)
			if err != nil {
				t.Fatalf("GetEmailListData %d (list:%v): %v", i, tc.emailListData.ListId, err)
			}

			if got.ListId != tc.emailListData.ListId {
				t.Fatalf("ListId mismatch: got %v want %v", got.ListId, tc.emailListData.ListId)
			}

			if got.Name != tc.emailListData.Name {
				t.Fatalf("Name mismatch: got %q want %q", got.Name, tc.emailListData.Name)
			}
		})
	}

	for i, tc := range tests {
		ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
		defer cancel()
		if _, err := emailRepo.AddToEmailList(ctx, tc.emailData.ListId, tc.subscribers); err != nil {
			t.Fatalf("add email to list %d: %v", i, err)
		}
	}

	for i, tc := range tests {
		i, tc := i, tc
		t.Run(fmt.Sprintf("%02d/GetEmailListSubscribers", i), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			got, err := emailRepo.GetEmailListSubscribers(ctx, tc.emailData.ListId)
			if err != nil {
					t.Fatalf("GetEmailListSubscribers %d (list:%v): %v", i, tc.emailData.ListId, err)
			}

			if len(got) != len(tc.subscribers) {
					t.Fatalf("subscriber list length mismatch: got %d want %d", len(got), len(tc.subscribers))
			}

			for j := range len(got) {
				gotSub, sub := got[j], tc.subscribers[j]
				if gotSub.Id != sub.Id {
					t.Fatalf("Id mismatch: got %v want %v", gotSub.Id, sub.Id)
				}

				if gotSub.FirstName != sub.FirstName {
					t.Fatalf("FirstName mismatch: got %s want %s", gotSub.FirstName, sub.FirstName)
				}

				if gotSub.LastName != sub.LastName {
					t.Fatalf("LastName mismatch: got %s want %s", gotSub.LastName, sub.LastName)
				}

				if gotSub.Email != sub.Email {
					t.Fatalf("Email mismatch: got %s want %s", gotSub.Email, sub.Email)
				}

				if gotSub.ListId != sub.ListId {
					t.Fatalf("ListId mismatch: got %v want %v", gotSub.ListId, sub.ListId)
				}

				if gotSub.IsSubscribed != sub.IsSubscribed {
					t.Fatalf("IsSubscribed mismatch: got %t want %t", gotSub.IsSubscribed, sub.IsSubscribed)
				}
			}
		})
	}
}

