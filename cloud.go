package slackpaul

import (
	"database/sql"
	"fmt"
	"github.com/CedricFinance/slackpaul/application"
	"github.com/CedricFinance/slackpaul/config"
	"github.com/CedricFinance/slackpaul/database"
	"github.com/CedricFinance/slackpaul/domain/entities"
	"github.com/CedricFinance/slackpaul/infrastructure/repository"
	"github.com/mattn/go-shellwords"
	"github.com/nlopes/slack"
	"github.com/nlopes/slack/slackevents"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const helpMessage = "To start a poll, type this: `/paul Question Choice1 Choice2 ...`\nA poll must have at least a title and two choices.\n:warning: Put you question and each choice between \"\" if they contain spaces.\nBefore the question, you can add options to configure the poll.\nThe available options are:\n- `limit X` to limit the number of votes per user. The default value is 1.\n- `max X` to limit the number of votes per choice. The default value is 0 (unlimited).\n- `anonymous` to make the poll anonymous"

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
	rand.Seed(time.Now().Unix())

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

	if len(args) == 0 {
		application.WriteMessage(w, helpMessage)
		return
	}

	poll, err := ConfigurePoll(args)
	if err != nil {
		application.WriteMessage(w, fmt.Sprintf("Sorry, %s\n%s", err.Error(), helpMessage))
		return
	}

	err = repo.SavePoll(r.Context(), db, poll)
	if err != nil {
		application.WriteError(w, err)
		return
	}

	msg := FormatQuestionAlt(poll, nil)

	application.WriteJSON(w, msg)
}

func ConfigurePoll(args []string) (entities.Poll, error) {
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

	poll := entities.NewPoll(args[0], args[1:])
	poll.MaxVotes = maxVotes
	poll.Anonymous = anonymous
	poll.MaxVotesPerProposition = maxVotesPerProposition

	return poll, nil
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

	pollId := messageAction.CallbackID
	userId := messageAction.User.ID

	poll, err := repo.FindPollByID(r.Context(), db, pollId)
	if err != nil {
		application.WriteError(w, err)
		return
	}

	var selectedProposition int
	if poll.Propositions[0] == "BlaBlaPoll" && rand.Intn(100) <= 75 {
		selectedProposition = 0
	} else {
		selectedProposition, err = strconv.Atoi(messageAction.Actions[0].Value)
		if err != nil {
			application.WriteError(w, err)
			return
		}
	}

	votes, err := repo.GetAllVotes(r.Context(), pollId)
	if err != nil {
		application.WriteError(w, err)
		return
	}

	newVote := entities.NewVote(userId, pollId, selectedProposition)
	userVotes := UserVotes(votes, userId)
	if len(userVotes) < poll.MaxVotes {

		if poll.MaxVotesPerProposition != 0 && len(PropositionVotes(votes, selectedProposition)) >= poll.MaxVotesPerProposition {
			application.WriteMessage(w, "Sorry, this choice has too many votes")
			return
		}

		err = repo.SaveVote(r.Context(), newVote)
		if err != nil {
			application.WriteError(w, err)
			return
		}

		votes = append(votes, &newVote)
	} else {
		// allow to change the vote when we can vote only once (need to think how to do it for polls with multiple votes)
		if poll.MaxVotes == 1 {
			userVotes[0].ChangeSelectedProposition(selectedProposition)
			if err != nil {
				application.WriteError(w, err)
				return
			}
		} else {
			application.WriteMessage(w, fmt.Sprintf("Sorry, you already have voted %d times", poll.MaxVotes))
			return
		}

	}

	msg := FormatQuestionAlt(poll, votes)
	msg.ReplaceOriginal = true
	application.WriteJSON(w, msg)
}

func PropositionVotes(votes []*entities.Vote, propositionId int) []*entities.Vote {
	var results []*entities.Vote
	for _, vote := range votes {
		if vote.SelectedProposition == propositionId {
			results = append(results, vote)
		}
	}
	return results
}

func UserVotes(votes []*entities.Vote, userId string) []*entities.Vote {
	var userVotes []*entities.Vote
	for _, vote := range votes {
		if vote.UserId == userId {
			userVotes = append(userVotes, vote)
		}
	}
	return userVotes
}

type SymbolsSource interface {
	ForIndex(i int) string
}

type ArraySymbolsSource []string

func (a ArraySymbolsSource) ForIndex(i int) string {
	if i < len(a) {
		return a[i]
	}
	return fmt.Sprintf("%d", i+1)
}

type NumbersSymbolsSource struct{}

func (NumbersSymbolsSource) ForIndex(i int) string {
	return fmt.Sprintf("%d", i+1)
}

type VoteFormatter func(vote *entities.Vote) string

func UserVoteFormatter(vote *entities.Vote) string {
	return fmt.Sprintf("<@%s>", vote.UserId)
}

func AnonymousVoteFormatter(vote *entities.Vote) string {
	return ":thumbsup:"
}

func FormatQuestionAlt(poll entities.Poll, votes []*entities.Vote) slack.Msg {
	votes = votes[:]
	sort.Sort(ByUpdateDate(votes))
	votesByProposition := make([][]*entities.Vote, len(poll.Propositions))
	for _, vote := range votes {
		votesByProposition[vote.SelectedProposition] = append(votesByProposition[vote.SelectedProposition], vote)
	}

	symbols := GetSymbolsSource(poll)
	formatter := GetVoteFormatter(poll)

	msg := slack.Msg{}

	var explanations []string
	if poll.Anonymous {
		explanations = append(explanations, ":bust_in_silhouette: This poll is anonymous.")
	}

	if poll.MaxVotes > 1 {
		explanations = append(explanations, fmt.Sprintf("You can vote up to %d times.", poll.MaxVotes))
	}

	if poll.MaxVotesPerProposition != 0 {
		explanations = append(explanations, fmt.Sprintf("Choices can have up to %d votes.", poll.MaxVotesPerProposition))
	}

	if len(explanations) > 0 {
		msg.Text = strings.Join(explanations, " ")
	}

	buttonsAttachmentsCount := int(math.Ceil(float64(len(poll.Propositions)) / 5))

	msg.Attachments = make([]slack.Attachment, 0, 1+buttonsAttachmentsCount)

	propositionsFields := make([]slack.AttachmentField, len(poll.Propositions))
	for i, proposition := range poll.Propositions {
		voters := make([]string, len(votesByProposition[i]))
		for i, vote := range votesByProposition[i] {
			voters[i] = formatter(vote)
		}
		propositionsFields[i] = slack.AttachmentField{
			Value: fmt.Sprintf("*%s* %s    `%d`\n%s", symbols.ForIndex(i), mdEscape(proposition), len(votesByProposition[i]), strings.Join(voters, " "))}
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
			actions[j] = slack.AttachmentAction{Name: "vote", Type: "button", Value: fmt.Sprintf("%d", j+lowerBound), Text: symbols.ForIndex(j + lowerBound)}
		}

		msg.Attachments = append(msg.Attachments, slack.Attachment{
			Actions:    actions,
			CallbackID: poll.Id,
		})

	}

	msg.ResponseType = "in_channel"

	return msg
}

func mdEscape(s string) string {
	return ""
}

func GetSymbolsSource(poll entities.Poll) SymbolsSource {
	if len(poll.Propositions) <= 9 {
		return ArraySymbolsSource(PropositionsEmojis)
	}

	return NumbersSymbolsSource{}
}

func GetVoteFormatter(poll entities.Poll) VoteFormatter {
	if poll.Anonymous {
		return AnonymousVoteFormatter
	}

	return UserVoteFormatter
}

type ByUpdateDate []*entities.Vote

func (d ByUpdateDate) Len() int {
	return len(d)
}

func (d ByUpdateDate) Less(i, j int) bool {
	return d[i].UpdatedAt.Before(d[j].CreatedAt)
}

func (d ByUpdateDate) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}

func Sanitize(str string) string {
	return strings.Replace(strings.Replace(str, "“", "\"", -1), "”", "\"", -1)
}
