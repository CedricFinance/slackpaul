package application

import (
	"encoding/json"
	"fmt"
	"github.com/CedricFinance/slackpaul/domain/entities"
	"github.com/nlopes/slack"
	"net/http"
	"strconv"
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

func ConfigurePoll(args []string, channelId string, ownerId string) (entities.Poll, error) {
	maxVotes := 1
	maxVotesPerProposition := 0
	anonymous := false

	optionFound := true
	for optionFound && len(args) > 0 {
		optionFound = false

		if args[0] == "limit" && len(args) >= 5 {
			var err error
			maxVotes, err = strconv.Atoi(args[1])
			if err != nil {
				return entities.Poll{}, fmt.Errorf("%q is not a valid value for the max number of vote per participant", args[1])
			}
			args = args[2:]
			optionFound = true
		}

		if args[0] == "max" && len(args) >= 5 {
			var err error
			maxVotesPerProposition, err = strconv.Atoi(args[1])
			if err != nil {
				return entities.Poll{}, fmt.Errorf("%q is not a valid value for the max number of vote per choice", args[1])
			}
			args = args[2:]
			optionFound = true
		}

		if args[0] == "anonymous" && len(args) >= 4 {
			anonymous = true

			args = args[1:]
			optionFound = true
		}

	}

	if len(args) < 3 {
		return entities.Poll{}, fmt.Errorf("a poll must have at least a title and two choices")
	}

	title := args[0]
	propositions := args[1:]

	if maxVotes < 1 {
		maxVotes = len(propositions) + maxVotes
	}

	poll := entities.NewPoll(title, propositions, channelId, ownerId)
	poll.MaxVotes = maxVotes
	poll.Anonymous = anonymous
	poll.MaxVotesPerProposition = maxVotesPerProposition

	return poll, nil
}
