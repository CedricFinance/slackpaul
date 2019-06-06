package blablapoll

import (
	"database/sql"
	"fmt"
	"github.com/CedricFinance/blablapoll/application"
	"github.com/CedricFinance/blablapoll/config"
	"github.com/CedricFinance/blablapoll/database"
	"github.com/CedricFinance/blablapoll/domain/entities"
	"github.com/CedricFinance/blablapoll/infrastructure/repository"
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
	repo := repository.New(db)

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

	args, err := shellwords.Parse(Sanitize(slashCommand.Text))
	if err != nil {
		application.WriteError(w, err)
		return
	}

	poll := entities.NewPoll(args[0], args[1:])

	err = repo.SavePoll(r.Context(), db, poll)
	if err != nil {
		application.WriteError(w, err)
		return
	}

	//msg := FormatQuestion(question, propositions)
	msg := FormatQuestionAlt(poll, nil)

	application.WriteJSON(w, msg)
}

func OnActionTrigger(w http.ResponseWriter, r *http.Request) {
	repo := repository.New(db)

	r.ParseForm()

	payload := r.Form.Get("payload")
	messageAction, err := slackevents.ParseActionEvent(payload, slackevents.OptionNoVerifyToken())
	if err != nil {
		panic(err)
		application.WriteError(w, err)
		return
	}

	selectedProposition, err := strconv.Atoi(messageAction.Actions[0].Value)
	if err != nil {
		application.WriteError(w, err)
		return
	}

	pollId := messageAction.CallbackID
	userId := messageAction.User.ID

	votes, err := repo.GetAllVotes(r.Context(), pollId)
	if err != nil {
		application.WriteError(w, err)
		return
	}

	poll, err := repo.FindPollByID(r.Context(), db, pollId)
	if err != nil {
		application.WriteError(w, err)
		return
	}

	newVote := entities.NewVote(userId, pollId, selectedProposition)
	if len(UserVotes(votes, userId)) < poll.MaxVotes {
		err = repo.SaveVote(r.Context(), newVote)
		if err != nil {
			application.WriteError(w, err)
			return
		}
	} else {
		application.WriteMessage(w, fmt.Sprintf("Sorry, you already have voted %d times", poll.MaxVotes))
		return
	}

	votes = append(votes, newVote)
	msg := FormatQuestionAlt(poll, votes)
	msg.ReplaceOriginal = true
	application.WriteJSON(w, msg)
}

func UserVotes(votes []entities.Vote, userId string) []entities.Vote {
	var userVotes []entities.Vote
	for _, vote := range votes {
		if vote.UserId == userId {
			userVotes = append(userVotes, vote)
		}
	}
	return userVotes
}

func FormatQuestionAlt(poll entities.Poll, votes []entities.Vote) slack.Msg {
	votes = votes[:]
	sort.Sort(ByCreationDate(votes))
	votesByProposition := make([][]entities.Vote, len(poll.Propositions))
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
			actions[j] = slack.AttachmentAction{Name: "vote", Type: "button", Value: fmt.Sprintf("%d", j+lowerBound), Text: PropositionsEmojis[j+lowerBound]}
		}

		msg.Attachments = append(msg.Attachments, slack.Attachment{
			Actions:    actions,
			CallbackID: poll.Id,
		})

	}

	msg.ResponseType = "in_channel"

	return msg
}

type ByCreationDate []entities.Vote

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

func Sanitize(str string) string {
	return strings.Replace(strings.Replace(str, "“", "\"", -1), "”", "\"", -1)
}
