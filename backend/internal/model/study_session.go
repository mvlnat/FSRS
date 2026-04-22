package model

type StudySession struct {
	DueCards             []CardWithState `json:"due_cards"`
	PendingLearningCards []CardWithState `json:"pending_learning_cards"`
}
