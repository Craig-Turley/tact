package utils

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"
)

var TIME_FORMAT = time.RFC3339

var (
	ERROR_INVALID_RETRY_LIMIT   = "Error invalid retry limit"
	ERROR_JOB_TYPE_INVALID      = "Error invalid job type"
	ERROR_JOB_NAME_NOT_PROVIDED = "Error job name not provided"
	ERROR_JOB_RUNNER_NOT_FOUND  = "Error associated job runner not found"
	ERROR_JOB_TYPE_MISMATCH     = "Error runner and job type mismatch. Got %d expected %d"
	ERROR_JOB_FAILED            = "Error job failed"
	ERROR_JOB_STATUS_INVALID    = "Error job status invalid"
)

func NewError(template string, args ...any) error {
	if countFormats(template) != len(args) {
		return errors.New("Error: No information available - error generating message")
	}
	return fmt.Errorf(fmt.Sprintf(template, args...))
}

var FORMAT_REGEX = regexp.MustCompile(`%[dfsuXxobegt]`)

func countFormats(format string) int {
	return len(FORMAT_REGEX.FindAllString(format, -1))
}

func Assert(statement string, conditions ...bool) {
	for i, condition := range conditions {
		if !condition {
			panic(fmt.Sprintf("Assert failed: %s (condition #%d)", statement, i+1))
		}
	}
}

func ValidEmailListName(name string) bool {
	return true
}

func MergeJson(json1, json2 []byte) []byte {
	var m1, m2 map[string]any

	json.Unmarshal(json1, &m1)
	json.Unmarshal(json2, &m2)

	for k, v := range m2 {
		m1[k] = v
	}

	mergedJson, _ := json.MarshalIndent(m1, "", " ")
	return mergedJson
}

func WithTx(db *sql.DB, ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	return nil
}

func Getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
