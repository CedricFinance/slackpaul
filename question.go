package blablapoll

import "github.com/google/uuid"

type Poll struct {
    Id string
    Title string
    Propositions []string
}

func NewPoll(title string, propositions []string) Poll {
    return Poll {
        Id: uuid.New().String(),
        Title: title,
        Propositions: propositions,
    }
}
