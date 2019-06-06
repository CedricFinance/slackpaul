package blablapoll

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/CedricFinance/blablapoll/config"
	"github.com/CedricFinance/blablapoll/database"
	"github.com/mattn/go-shellwords"
	"github.com/nlopes/slack"
	"github.com/nlopes/slack/slackevents"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

func ParseSlashCommand(r *http.Request) (slack.SlashCommand, error) {
	return slack.SlashCommandParse(r)
}

func SecureParseSlashCommand(r *http.Request, signingSecret string) (slack.SlashCommand, error) {
	verifier, err := slack.NewSecretsVerifier(r.Header, signingSecret)
	if err != nil {
		return slack.SlashCommand{}, err
	}

	r.Body = ioutil.NopCloser(io.TeeReader(r.Body, &verifier))

	slashCommand, err := slack.SlashCommandParse(r)
	if err != nil {
		return slack.SlashCommand{}, err
	}

	if err = verifier.Ensure(); err != nil {
		return slack.SlashCommand{}, err
	}

	return slashCommand, nil
}

var PropositionsEmojis = []string{
	":one:", ":two:", ":three:", ":four:", ":five:", ":six:", ":seven:", ":eight:", ":nine:",
}

var db *sql.DB

func init() {
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
}

func OnSlashCommandTrigger(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprint(w, err)
		return
	}

	var slashCommand slack.SlashCommand
	if r.Host == "localhost:8080" {
		slashCommand, err = ParseSlashCommand(r)
	} else {
		slashCommand, err = SecureParseSlashCommand(r, cfg.GetSlackSigningSecret())
	}

	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	_ = slashCommand.Text

	args, err := shellwords.Parse(Sanitize(slashCommand))
	if err != nil {
		WriteError(w, err)
		return
	}

	poll := NewPoll(args[0], args[1:])

	err = SavePoll(r.Context(), db, poll)
	if err != nil {
		WriteError(w, err)
		return
	}

	//msg := FormatQuestion(question, propositions)
	msg := FormatQuestionAlt(poll, nil)

	WriteJSON(w, msg)
}

func SavePoll(context context.Context, db *sql.DB, poll Poll) error {
	propositions, err := json.Marshal(poll.Propositions)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(
		context,
		"INSERT INTO polls(id,title,propositions,created_at) VALUES(?,?,?,?)",
		poll.Id,
		poll.Title,
		propositions,
		time.Now().UTC(),
	)

	return err
}

func SaveVote(context context.Context, db *sql.DB, vote Vote) error {
	_, err := db.ExecContext(
		context,
		"INSERT INTO votes(id,poll_id,user_id,selected_proposition,created_at) VALUES(?,?,?,?,?)",
		vote.Id,
		vote.PollId,
		vote.UserId,
		vote.SelectedProposition,
		vote.CreatedAt,
	)

	return err
}

type dbPoll struct {
	Id           string
	Title        string
	Propositions []byte
	CreatedAt    time.Time
}

type PollNotFound struct {
	ID string
}

func (e PollNotFound) Error() string {
	return fmt.Sprintf("no poll with id %q", e.ID)
}

func GetAllVotes(context context.Context, db *sql.DB, pollId string) ([]Vote, error) {
	rows, err := db.QueryContext(context, "SELECT id,user_id,selected_proposition,created_at FROM votes WHERE poll_id=?", pollId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Vote

	for rows.Next() {
		var voteId string
		var userId string
		var selectedProposition int
		var createdAt time.Time

		err = rows.Scan(&voteId, &userId, &selectedProposition, &createdAt)

		if err != nil {
			return results, err
		}

		results = append(results, Vote{Id: voteId, UserId: userId, SelectedProposition: selectedProposition, CreatedAt: createdAt})
	}

	return results, nil
}

func FindPollByID(context context.Context, db *sql.DB, id string) (Poll, error) {
	rows, err := db.QueryContext(context, "SELECT id,title,propositions,created_at FROM polls WHERE id=?", id)
	if err != nil {
		return Poll{}, err
	}
	defer rows.Close()

	if !rows.Next() {
		return Poll{}, PollNotFound{ID: id}
	}

	var p dbPoll
	err = rows.Scan(&p.Id, &p.Title, &p.Propositions, &p.CreatedAt)
	if err != nil {
		return Poll{}, err
	}

	var props []string
	err = json.Unmarshal(p.Propositions, &props)
	if err != nil {
		return Poll{}, err
	}

	return Poll{
		Id:           p.Id,
		Title:        p.Title,
		Propositions: props,
		CreatedAt:    p.CreatedAt,
	}, nil
}

func OnActionTrigger(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	payload := r.Form.Get("payload")
	messageAction, err := slackevents.ParseActionEvent(payload, slackevents.OptionNoVerifyToken())
	if err != nil {
		panic(err)
		WriteError(w, err)
		return
	}

	poll, err := FindPollByID(r.Context(), db, messageAction.CallbackID)
	if err != nil {
		WriteError(w, err)
		return
	}

	selectedProposition, err := strconv.Atoi(messageAction.Actions[0].Value)
	if err != nil {
		WriteError(w, err)
		return
	}

	userId := messageAction.User.ID
	err = SaveVote(r.Context(), db, NewVote(userId, poll.Id, selectedProposition))
	if err != nil {
		WriteError(w, err)
		return
	}

	votes, err := GetAllVotes(r.Context(), db, poll.Id)
	if err != nil {
		WriteError(w, err)
		return
	}

	msg := FormatQuestionAlt(poll, votes)
	msg.ReplaceOriginal = true
	WriteJSON(w, msg)
}

func FormatQuestionAlt(poll Poll, votes []Vote) slack.Msg {
	votes = votes[:]
	sort.Sort(ByCreationDate(votes))
	votesByProposition := make([][]Vote, len(poll.Propositions))
	for _, vote := range votes {
		votesByProposition[vote.SelectedProposition] = append(votesByProposition[vote.SelectedProposition], vote)
	}

	msg := slack.Msg{}

	buttonsAttachmentsCount := int(math.Ceil(float64(len(poll.Propositions)) / 5))

	msg.Attachments = make([]slack.Attachment, 0, 1+buttonsAttachmentsCount)

	propositionsFields := make([]slack.AttachmentField, len(poll.Propositions))
	for i, proposition := range poll.Propositions {
		voters := make([]string, len(votesByProposition[i]))
		for i, vote := range votesByProposition[i] {
			voters[i] = fmt.Sprintf("<@%s>", vote.UserId)
		}
		propositionsFields[i] = slack.AttachmentField{
			Value: fmt.Sprintf("%s %s    `%d`\n%s", PropositionsEmojis[i], proposition, len(votesByProposition[i]), strings.Join(voters, " "))}
	}

	msg.Attachments = append(msg.Attachments, slack.Attachment{
		Title:  poll.Title,
		Fields: propositionsFields,
	})

	for i := 0; i < buttonsAttachmentsCount; i++ {
		lowerBound := 5 * i
		upperBound := int(math.Min(float64(len(poll.Propositions)), float64(lowerBound)+5))
		itemsCount := upperBound - lowerBound

		actions := make([]slack.AttachmentAction, upperBound-lowerBound)
		for j := 0; j < itemsCount; j++ {
			actions[j] = slack.AttachmentAction{Name: "vote", Type: "button", Value: fmt.Sprintf("%d", i), Text: PropositionsEmojis[j+lowerBound]}
		}

		msg.Attachments = append(msg.Attachments, slack.Attachment{
			Actions:    actions,
			CallbackID: poll.Id,
		})

	}

	return msg
}

type ByCreationDate []Vote

func (d ByCreationDate) Len() int {
	return len(d)
}

func (d ByCreationDate) Less(i, j int) bool {
	return d[i].CreatedAt.Before(d[j].CreatedAt)
}

func (d ByCreationDate) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}

func FormatQuestion(question string, propositions []string) slack.Message {
	questionText := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*%s*", question), false, false)
	questionSection := slack.NewSectionBlock(questionText, nil, nil)
	msg := slack.NewBlockMessage(
		questionSection,
	)
	// var text = make([]string, len(propositions))
	var buttons = make([]slack.BlockElement, len(propositions))
	for i, proposition := range propositions {
		propositionsText := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("%s %s", PropositionsEmojis[i], proposition), false, false)
		propositionsSection := slack.NewSectionBlock(propositionsText, nil, nil)
		//       text[i] = fmt.Sprintf("%s %s", PropositionsEmojis[i], proposition)
		msg = slack.AddBlockMessage(msg, propositionsSection)

		buttonText := slack.NewTextBlockObject("plain_text", PropositionsEmojis[i], true, false)
		buttons[i] = slack.NewButtonBlockElement(fmt.Sprintf("select-proposition-%d", i), fmt.Sprintf("%d", i), buttonText)

	}
	/*
	   propositionsText := slack.NewTextBlockObject("mrkdwn", strings.Join(text, "\n"), false, false)
	   propositionsSection := slack.NewSectionBlock(propositionsText, nil, nil)
	*/
	var buttonsSections = make([]slack.Block, int(math.Ceil(float64(len(buttons))/5)))
	for i := range buttonsSections {
		lowerBound := 5 * i
		upperBound := int(math.Min(float64(len(buttons)), float64(lowerBound)+5))
		buttonsSection := slack.NewActionBlock(fmt.Sprintf("select-propositions-%d", i), buttons[lowerBound:upperBound]...)

		msg = slack.AddBlockMessage(msg, buttonsSection)
	}
	return msg
}

func Sanitize(slashCommand slack.SlashCommand) string {
	return strings.Replace(strings.Replace(slashCommand.Text, "“", "\"", -1), "”", "\"", -1)
}

func WriteError(w http.ResponseWriter, err error) {
	msg := slack.Msg{
		ResponseType: "ephemeral",
		Text:         fmt.Sprintf("Sorry, an error occured: %s", err),
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
