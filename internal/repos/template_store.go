package repos

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Craig-Turley/task-scheduler.git/pkg/common/template"
	"github.com/bwmarrin/snowflake"
	"github.com/cespare/xxhash/v2"
)

// returns the 64-bit xxhash digest of x with a zero seed
func HashXX(x snowflake.ID) uint64 {
	return xxhash.Sum64String(strconv.FormatInt(int64(x), 10))
}

// retuns the constructed path of given digest. starts with a /
func ConstructPath(digest uint64) string {
	hash := fmt.Sprintf("%x", digest)
	path := fmt.Sprintf("%s/%s/%s.html", hash[0:4], hash[4:8], hash[8:16])
	return path
}

type TemplateStore interface {
	SaveTemplate(jobId snowflake.ID, t string) error
	GetTemplate(jobId snowflake.ID) (template.Template, error)
}

// not using pointers for now since data doesnt need to persist. we will see about this later
type LocalTemplateStore struct {
	dir string
}

func NewLocalTemplateStore(dir string) LocalTemplateStore {
	return LocalTemplateStore{
		dir: dir,
	}
}

func (l LocalTemplateStore) SaveTemplate(jobId snowflake.ID, t string) error {
	path := fmt.Sprintf("%s/%s", l.dir, ConstructPath(HashXX(jobId)))

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	file.Write([]byte(t))

	return nil
}

func (l LocalTemplateStore) GetTemplate(jobId snowflake.ID) (template.Template, error) {
	path := fmt.Sprintf("%s/%s", l.dir, ConstructPath(HashXX(jobId)))

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return template.Template(data), nil
}
