package main

import (
	"github.com/CedricFinance/blablapoll"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/commands", blablapoll.OnSlashCommandTrigger)
	http.HandleFunc("/actions", blablapoll.OnActionTrigger)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
