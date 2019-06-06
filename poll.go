package blablapoll

import (
	"github.com/google/uuid"
	"time"
)

type Poll struct {
	Id           string
	Title        string
	Propositions []string
	CreatedAt    time.Time
}

type Vote struct {
	Id                  string
	UserId              string
	PollId              string
	SelectedProposition int
	CreatedAt           time.Time
}

func NewPoll(title string, propositions []string) Poll {
	return Poll{
		Id:           uuid.New().String(),
		Title:        title,
		Propositions: propositions,
	}
}

func NewVote(userId string, pollId string, selectedProposition int) Vote {
	return Vote{
		Id:                  uuid.New().String(),
		UserId:              userId,
		PollId:              pollId,
		SelectedProposition: selectedProposition,
		CreatedAt:           time.Now().UTC(),
	}
}
