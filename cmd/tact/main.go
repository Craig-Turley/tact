package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/Craig-Turley/task-scheduler.git/internal/db"
	"github.com/Craig-Turley/task-scheduler.git/internal/repos"
	"github.com/Craig-Turley/task-scheduler.git/internal/services"
	"github.com/Craig-Turley/task-scheduler.git/pkg/idgen"
	"github.com/Craig-Turley/task-scheduler.git/pkg/utils"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	godotenv.Load()

	log.Println(utils.TIME_FORMAT)
	log.Println(len(utils.TIME_FORMAT))
	utils.Assert("Time format must be set", len(utils.TIME_FORMAT) > 0, utils.TIME_FORMAT == os.Getenv("TIME_FORMAT"))

	nodeStr := os.Getenv("NODE_ID")
	node, err := strconv.ParseInt(nodeStr, 10, 64)
	if err != nil {
		panic("Couldn't initialize snowflake id node")
	}

	if err := idgen.Init(node); err != nil {
		log.Panicf("Error initializing snowflake node %s", err)
	}

	sqlite3db := db.NewSqliteDb(os.Getenv("SQLITE_DB_PATH"))

	emailRepo := repos.NewSqliteEmailRepo(sqlite3db)
	templateStore := repos.NewLocalTemplateStore(os.Getenv("TEMPLATE_DIR"))
	emailSrvc := services.NewEmailService(emailRepo, templateStore)

	jobRepo := repos.NewSqliteJobRepo(sqlite3db)
	jobSrvc := services.NewJobService(jobRepo)

	// j := NewJob("test job", "* * * * * *", 3, TypeEmail)
	// jobSrvc.CreateJob(j)
	//
	// e := &EmailData{
	// 	JobId:  j.Id,
	// 	ListId: 1,
	// }
	//
	// emailSrvc.CreateEmailJob(e)

	schedulingRepo := repos.NewSqliteSchedulingRepo(sqlite3db)
	schedulingSrvc := services.NewSchedulingService(schedulingRepo)

	// if err := schedulingService.ScheduleJob(j.Id, j.Cron); err != nil {
	// 	log.Println(err)
	// }

	tick, err := time.ParseDuration(os.Getenv("TICK"))
	if err != nil {
		panic(err)
	}

	executorSrvc := services.NewExecutorService(tick, jobSrvc, emailSrvc, schedulingSrvc)
	executorSrvc.Start()

	select {}
}
