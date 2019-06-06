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
	"io"
	"io/ioutil"
	"math"
	"net/http"
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

func OnSlashCommandTrigger(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprint(w, err)
		return
	}
	_ = cfg

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
		writeError(w, err)
		return
	}

	poll := NewPoll(args[0], args[1:])
	db := database.Connect(
		cfg.GetDBUsername(),
		cfg.GetDBPassword(),
		cfg.GetDBName(),
		cfg.GetDBInstance())

	SavePoll(r.Context(), db, poll)

	//msg := FormatQuestion(question, propositions)
	msg := FormatQuestionAlt(poll)

	res, err := json.Marshal(msg)
	if err != nil {
		fmt.Fprint(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(res)
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

func OnActionTrigger(w http.ResponseWriter, r *http.Request) {
	body, _ := ioutil.ReadAll(r.Body)

	fmt.Println(string(body))

	defer r.Body.Close()

	fmt.Fprint(w, string(body))
}

func FormatQuestionAlt(poll Poll) slack.Msg {
	msg := slack.Msg{}

	buttonsAttachmentsCount := int(math.Ceil(float64(len(poll.Propositions)) / 5))

	msg.Attachments = make([]slack.Attachment, 0, 1+buttonsAttachmentsCount)

	propositionsFields := make([]slack.AttachmentField, len(poll.Propositions))
	for i, proposition := range poll.Propositions {
		propositionsFields[i] = slack.AttachmentField{Value: fmt.Sprintf("%s %s", PropositionsEmojis[i], proposition)}
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

func writeError(w http.ResponseWriter, err error) {
	msg := slack.Msg{
		ResponseType: "ephemeral",
		Text:         fmt.Sprintf("Sorry, an error occured: %s", err),
	}
	res, err := json.Marshal(msg)
	if err != nil {
		fmt.Fprint(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(res)
}
