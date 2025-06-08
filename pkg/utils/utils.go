package utils

import (
	"errors"
	"fmt"
	"os"
	"regexp"
)

var TIME_FORMAT = os.Getenv("TIME_FORMAT")

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
	return errors.New(fmt.Sprintf(template, args...))
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
