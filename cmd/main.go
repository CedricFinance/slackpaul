package main

import (
    "github.com/CedricFinance/blablapoll"
    "log"
    "net/http"
)

func main() {
    http.HandleFunc("/", blablapoll.OnSlashCommandTrigger)
    log.Fatal(http.ListenAndServe(":8080", nil))
}
