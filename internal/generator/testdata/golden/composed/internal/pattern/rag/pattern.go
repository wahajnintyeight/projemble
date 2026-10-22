package rag

import "context"

type Document struct {
	ID, Source, Text string
}

type Retriever interface {
	Retrieve(context.Context, string) ([]Document, error)
}

type Answerer interface {
	Answer(context.Context, string, []Document) (string, error)
}

type Service struct {
	retriever Retriever
	answerer  Answerer
}

func New(retriever Retriever, answerer Answerer) Service {
	return Service{retriever: retriever, answerer: answerer}
}

func (s Service) Ask(ctx context.Context, question string) (string, []Document, error) {
	documents, err := s.retriever.Retrieve(ctx, question)
	if err != nil {
		return "", nil, err
	}
	answer, err := s.answerer.Answer(ctx, question, documents)
	return answer, documents, err
}
