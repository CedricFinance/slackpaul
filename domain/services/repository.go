package services

import (
	"context"
	"fmt"
	"github.com/CedricFinance/slackpaul/domain/entities"
)

type Repository interface {
	FindPollByID(context context.Context, id string) (entities.Poll, error)
	SavePoll(context context.Context, poll entities.Poll) error

	GetAllVotes(context context.Context, pollId string) ([]*entities.Vote, error)
	SaveVote(context context.Context, vote entities.Vote) error
	UpdateVote(context context.Context, vote entities.Vote) error
}

type PollNotFound struct {
	ID string
}

func (e PollNotFound) Error() string {
	return fmt.Sprintf("no poll with id %q", e.ID)
}
