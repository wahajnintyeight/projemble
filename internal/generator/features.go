package generator

import (
	"fmt"
	"strings"

	"projemble/internal/catalog"
	"projemble/internal/projectstore"
)

var databaseModules = map[string]string{
	catalog.CapabilitySQLite:   "modernc.org/sqlite v1.60.1",
	catalog.CapabilityPostgres: "github.com/jackc/pgx/v5 v5.11.0",
	catalog.CapabilityMySQL:    "github.com/go-sql-driver/mysql v1.10.1",
	catalog.CapabilityMongoDB:  "go.mongodb.org/mongo-driver/v2 v2.9.2",
}

func goModForProject(project projectstore.Project, module string, template catalog.Template) string {
	result := goMod(module, template)
	var requirements []string
	for _, capability := range project.Capabilities {
		if dependency := databaseModules[capability]; dependency != "" {
			requirements = append(requirements, "\t"+dependency)
		}
	}
	if len(requirements) > 0 {
		result += "\nrequire (\n" + strings.Join(requirements, "\n") + "\n)\n"
	}
	return result
}

func addProfileFeatures(files map[string]string, project projectstore.Project, module string) error {
	for _, id := range project.Capabilities {
		if !databaseCapability(id) {
			capability, ok := catalog.CapabilityByID(id)
			if !ok || !capability.Supported {
				return fmt.Errorf("capability %q is planned and cannot be generated yet", id)
			}
			continue
		}
		files["internal/storage/database.go"] = databaseAdapter(id)
	}
	for _, id := range project.PatternIDs {
		pattern, ok := catalog.PatternByID(id)
		if !ok {
			return fmt.Errorf("unknown application pattern %q", id)
		}
		files["internal/pattern/"+id+"/pattern.go"] = patternScaffold(pattern, module)
	}
	return nil
}

func databaseCapability(id string) bool {
	return id == catalog.CapabilitySQLite || id == catalog.CapabilityPostgres || id == catalog.CapabilityMySQL || id == catalog.CapabilityMongoDB
}

func databaseAdapter(id string) string {
	switch id {
	case catalog.CapabilitySQLite:
		return `package storage

import (
	"database/sql"
	_ "modernc.org/sqlite"
)

func Open(databaseURL string) (*sql.DB, error) {
	if databaseURL == "" { databaseURL = "projemble.db" }
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil { return nil, err }
	if err := db.Ping(); err != nil { db.Close(); return nil, err }
	return db, nil
}
`
	case catalog.CapabilityPostgres:
		return `package storage

import (
	"database/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func Open(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil { return nil, err }
	if err := db.Ping(); err != nil { db.Close(); return nil, err }
	return db, nil
}
`
	case catalog.CapabilityMySQL:
		return `package storage

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
)

func Open(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("mysql", databaseURL)
	if err != nil { return nil, err }
	if err := db.Ping(); err != nil { db.Close(); return nil, err }
	return db, nil
}
`
	case catalog.CapabilityMongoDB:
		return `package storage

import (
	"context"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func Open(ctx context.Context, databaseURL string) (*mongo.Client, error) {
	return mongo.Connect(options.Client().ApplyURI(databaseURL))
}
`
	default:
		return ""
	}
}

func patternScaffold(pattern catalog.Pattern, module string) string {
	packageName := strings.ReplaceAll(pattern.ID, "-", "_")
	switch pattern.ID {
	case catalog.PatternRAG:
		return `// Package rag defines retrieval and response boundaries for RAG applications.
package rag

import "context"

type Document struct { ID, Source, Text string }
type Retriever interface { Retrieve(context.Context, string) ([]Document, error) }
type Answerer interface { Answer(context.Context, string, []Document) (string, error) }
type Service struct { retriever Retriever; answerer Answerer }
func New(retriever Retriever, answerer Answerer) Service { return Service{retriever: retriever, answerer: answerer} }
func (s Service) Ask(ctx context.Context, question string) (string, []Document, error) {
	documents, err := s.retriever.Retrieve(ctx, question)
	if err != nil { return "", nil, err }
	answer, err := s.answerer.Answer(ctx, question, documents)
	return answer, documents, err
}
`
	case catalog.PatternAgent:
		return `// Package agent defines explicit, bounded tool invocation for agent applications.
package agent

import (
	"context"
	"fmt"
)

type Tool interface { Name() string; Run(context.Context, string) (string, error) }
type Runner struct { tools map[string]Tool }
func New(tools ...Tool) Runner {
	registry := make(map[string]Tool, len(tools))
	for _, tool := range tools { if tool != nil && tool.Name() != "" { registry[tool.Name()] = tool } }
	return Runner{tools: registry}
}
func (r Runner) Call(ctx context.Context, name, input string) (string, error) {
	tool, ok := r.tools[name]
	if !ok { return "", fmt.Errorf("unknown tool %q", name) }
	return tool.Run(ctx, input)
}
`
	case catalog.PatternChatbot:
		return `// Package chatbot separates conversation history from response generation.
package chatbot

import "context"

type Message struct { Role, Content string }
type Store interface { Append(context.Context, string, Message) error; History(context.Context, string) ([]Message, error) }
type Responder interface { Reply(context.Context, []Message) (string, error) }
type Service struct { store Store; responder Responder }
func New(store Store, responder Responder) Service { return Service{store: store, responder: responder} }
func (s Service) Send(ctx context.Context, id, text string) (string, error) {
	if err := s.store.Append(ctx, id, Message{Role: "user", Content: text}); err != nil { return "", err }
	history, err := s.store.History(ctx, id)
	if err != nil { return "", err }
	answer, err := s.responder.Reply(ctx, history)
	if err != nil { return "", err }
	if err := s.store.Append(ctx, id, Message{Role: "assistant", Content: answer}); err != nil { return "", err }
	return answer, nil
}
`
	}
	return fmt.Sprintf(`// Package %s defines the application boundary for the %s pattern.
package %s

import "context"

// Handler is the application-specific implementation supplied by the project.
type Handler interface {
	Handle(context.Context, string) (string, error)
}

// Service keeps transport and provider integrations outside the pattern contract.
type Service struct { handler Handler }

func New(handler Handler) Service { return Service{handler: handler} }
func (s Service) Handle(ctx context.Context, input string) (string, error) {
	return s.handler.Handle(ctx, input)
}
`, packageName, pattern.Name, packageName)
}
