package application

import (
	"encoding/json"
	"fmt"
	"github.com/nlopes/slack"
	"net/http"
)

func WriteError(w http.ResponseWriter, err error) {
	msg := slack.Msg{
		ResponseType: "ephemeral",
		Text:         fmt.Sprintf("Sorry, an error occured: %s", err),
	}

	WriteJSON(w, msg)
}

func WriteMessage(w http.ResponseWriter, message string) {
	msg := slack.Msg{
		ResponseType: "ephemeral",
		Text:         message,
	}

	WriteJSON(w, msg)
}

func WriteJSON(w http.ResponseWriter, d interface{}) {
	res, err := json.Marshal(d)
	if err != nil {
		fmt.Fprint(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(res)
}
