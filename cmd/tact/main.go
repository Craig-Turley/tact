package main

import (
	"log"
	"os"
	"strconv"

	"github.com/Craig-Turley/task-scheduler.git/internal/api"
	"github.com/Craig-Turley/task-scheduler.git/internal/auth"
	"github.com/Craig-Turley/task-scheduler.git/pkg/idgen"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	godotenv.Load()
	node, err := strconv.ParseInt(os.Getenv("NODE_ID"), 10, 32)
	if err != nil {
		panic(err)
	}

	idgen.Init(node)

	auth.NewAuth()
	server := api.NewServer(":8080")
	log.Println(server.Run())
}
