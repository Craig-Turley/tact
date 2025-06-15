package main

import (
	"log"

	"github.com/Craig-Turley/task-scheduler.git/internal/api"
)

// import (
// 	"log"
// 	"os"
// 	"strconv"
// 	"time"
//
// 	"github.com/Craig-Turley/task-scheduler.git/internal/db"
// 	"github.com/Craig-Turley/task-scheduler.git/internal/repos"
// 	"github.com/Craig-Turley/task-scheduler.git/internal/services"
// 	"github.com/Craig-Turley/task-scheduler.git/pkg/idgen"
// 	"github.com/joho/godotenv"
// 	_ "github.com/mattn/go-sqlite3"
// )
//
// func main() {
// 	godotenv.Load()
//
// 	nodeStr := os.Getenv("NODE_ID")
// 	node, err := strconv.ParseInt(nodeStr, 10, 64)
// 	if err != nil {
// 		panic("Couldn't initialize snowflake id node")
// 	}
//
// 	if err := idgen.Init(node); err != nil {
// 		log.Panicf("Error initializing snowflake node %s", err)
// 	}
//
// 	sqlite3db := db.NewSqliteDb(os.Getenv("SQLITE_DB_PATH"))
//
// 	emailRepo := repos.NewSqliteEmailRepo(sqlite3db)
// 	templateStore := repos.NewLocalTemplateStore(os.Getenv("TEMPLATE_DIR"))
// 	emailSrvc := services.NewEmailService(emailRepo, templateStore)
//
// 	jobRepo := repos.NewSqliteJobRepo(sqlite3db)
// 	jobSrvc := services.NewJobService(jobRepo)
//
// 	schedulingRepo := repos.NewSqliteSchedulingRepo(sqlite3db)
// 	schedulingSrvc := services.NewSchedulingService(schedulingRepo)
//
// 	workerSrvc := services.NewLocalWorkerService(schedulingSrvc, emailSrvc)
//
// 	tick, err := time.ParseDuration(os.Getenv("TICK"))
// 	if err != nil {
// 		panic(err)
// 	}
//
// 	executorSrvc := services.NewExecutorService(tick, jobSrvc, emailSrvc, schedulingSrvc, workerSrvc)
// 	executorSrvc.Start()
//
// 	select {}
// }

func main() {
	server := api.NewServer(":8080")

	log.Println(server.Run())
}
