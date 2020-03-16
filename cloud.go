package slackpaul

import (
	"database/sql"
	"fmt"
	"github.com/CedricFinance/slackpaul/application"
	"github.com/CedricFinance/slackpaul/config"
	"github.com/CedricFinance/slackpaul/database"
	"github.com/CedricFinance/slackpaul/infrastructure/repository"
	"math/rand"
	"net/http"
	"time"
)

var db *sql.DB
var server *application.Server

func init() {
	rand.Seed(time.Now().Unix())

	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Print(err)
		return
	}

	db = database.Connect(
		cfg.GetDBUsername(),
		cfg.GetDBPassword(),
		cfg.GetDBName(),
		cfg.GetDBHost())

	server = &application.Server{
		Repository: repository.New(db),
	}
}

func OnSlashCommandTrigger(w http.ResponseWriter, r *http.Request) {
	server.HandleSlashCommand(w, r)
}

func OnActionTrigger(w http.ResponseWriter, r *http.Request) {
	server.HandleAction(w, r)
}
