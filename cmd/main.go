package main

import (
	"github.com/CedricFinance/slackpaul"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/commands", slackpaul.OnSlashCommandTrigger)
	http.HandleFunc("/actions", slackpaul.OnActionTrigger)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
