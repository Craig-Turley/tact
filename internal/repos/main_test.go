package repos_test

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/Craig-Turley/task-scheduler.git/internal/db"
	"github.com/Craig-Turley/task-scheduler.git/pkg/idgen"
	"github.com/joho/godotenv"
)

var sqlite3db *sql.DB

func TestMain(m *testing.M) {
	godotenv.Load()

	// nodeStr := os.Getenv("NODE_ID")
	nodeStr := "1"
	node, err := strconv.ParseInt(nodeStr, 10, 64)
	if err != nil {
		panic("Couldn't initialize snowflake id node")
	}

	if err := idgen.Init(node); err != nil {
		panic(fmt.Sprintf("Error initializing snowflake node %s", err))
	}

	// TODO find a way to make this env a test env - fix this
	// sqlite3db = db.NewSqliteDb(os.Getenv("SQLITE_DB_PATH"))
	sqlite3db = db.NewSqliteDb("../../scheduling_test.db")

	sqlite3db.Exec(`PRAGMA journal_mode=WAL;`)
	sqlite3db.Exec(`PRAGMA busy_timeout=5000;`)
	
	code := m.Run()
	os.Exit(code)
}
