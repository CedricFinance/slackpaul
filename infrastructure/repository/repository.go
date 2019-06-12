package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/CedricFinance/blablapoll/domain/entities"
	"github.com/CedricFinance/blablapoll/domain/services"
	"time"
)

type repository struct {
	db *sql.DB
}

func New(db *sql.DB) services.Repository {
	return &repository{db: db}
}

type dbPoll struct {
	Id                     string
	Title                  string
	Propositions           []byte
	MaxVotes               int
	MaxVotesPerProposition int
	Anonymous              bool
	CreatedAt              time.Time
}

func (r *repository) FindPollByID(context context.Context, db *sql.DB, id string) (entities.Poll, error) {
	rows, err := db.QueryContext(context, "SELECT id,title,propositions,max_votes,max_by_proposition,anonymous,created_at FROM polls WHERE id=?", id)
	if err != nil {
		return entities.Poll{}, err
	}
	defer rows.Close()

	if !rows.Next() {
		return entities.Poll{}, services.PollNotFound{ID: id}
	}

	var p dbPoll
	err = rows.Scan(&p.Id, &p.Title, &p.Propositions, &p.MaxVotes, &p.MaxVotesPerProposition, &p.Anonymous, &p.CreatedAt)
	if err != nil {
		return entities.Poll{}, err
	}

	var props []string
	err = json.Unmarshal(p.Propositions, &props)
	if err != nil {
		return entities.Poll{}, err
	}

	return entities.Poll{
		Id:                     p.Id,
		Title:                  p.Title,
		Propositions:           props,
		MaxVotes:               p.MaxVotes,
		MaxVotesPerProposition: p.MaxVotesPerProposition,
		Anonymous:              p.Anonymous,
		CreatedAt:              p.CreatedAt,
	}, nil
}

func (r *repository) SavePoll(context context.Context, db *sql.DB, poll entities.Poll) error {
	propositions, err := json.Marshal(poll.Propositions)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(
		context,
		"INSERT INTO polls(id,title,propositions,max_votes,max_by_proposition,anonymous,created_at) VALUES(?,?,?,?,?,?,?)",
		poll.Id,
		poll.Title,
		propositions,
		poll.MaxVotes,
		poll.MaxVotesPerProposition,
		poll.Anonymous,
		time.Now().UTC(),
	)

	return err
}

func (r *repository) SaveVote(context context.Context, vote entities.Vote) error {
	_, err := r.db.ExecContext(
		context,
		"INSERT INTO votes(id,poll_id,user_id,selected_proposition,created_at,updated_at) VALUES(?,?,?,?,?,?)",
		vote.Id,
		vote.PollId,
		vote.UserId,
		vote.SelectedProposition,
		vote.CreatedAt,
		vote.UpdatedAt,
	)

	return err
}

func (r *repository) UpdateVote(context context.Context, vote entities.Vote) error {
	_, err := r.db.ExecContext(
		context,
		"UPDATE votes SET selected_proposition = ?, updated_at = ? WHERE id=?",
		vote.SelectedProposition,
		vote.UpdatedAt,
		vote.Id,
	)

	return err
}

func (r *repository) GetAllVotes(context context.Context, pollId string) ([]*entities.Vote, error) {
	rows, err := r.db.QueryContext(context, "SELECT id,user_id,selected_proposition,created_at FROM votes WHERE poll_id=?", pollId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*entities.Vote

	for rows.Next() {
		var voteId string
		var userId string
		var selectedProposition int
		var createdAt time.Time

		err = rows.Scan(&voteId, &userId, &selectedProposition, &createdAt)

		if err != nil {
			return results, err
		}

		results = append(results, &entities.Vote{Id: voteId, UserId: userId, SelectedProposition: selectedProposition, CreatedAt: createdAt})
	}

	return results, nil
}
