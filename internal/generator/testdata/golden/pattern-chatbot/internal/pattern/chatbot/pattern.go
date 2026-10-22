package chatbot

import "context"

type Message struct {
	Role, Content string
}

type Store interface {
	Append(context.Context, string, Message) error
	History(context.Context, string) ([]Message, error)
}

type Responder interface {
	Reply(context.Context, []Message) (string, error)
}

type Service struct {
	store     Store
	responder Responder
}

func New(store Store, responder Responder) Service {
	return Service{store: store, responder: responder}
}

func (s Service) Send(ctx context.Context, id, text string) (string, error) {
	if err := s.store.Append(ctx, id, Message{Role: "user", Content: text}); err != nil {
		return "", err
	}
	history, err := s.store.History(ctx, id)
	if err != nil {
		return "", err
	}
	answer, err := s.responder.Reply(ctx, history)
	if err != nil {
		return "", err
	}
	if err := s.store.Append(ctx, id, Message{Role: "assistant", Content: answer}); err != nil {
		return "", err
	}
	return answer, nil
}
