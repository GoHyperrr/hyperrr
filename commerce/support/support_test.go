package support

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/GoHyperrr/hyperrr/internal/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/config"
	"github.com/GoHyperrr/hyperrr/pkg/db"
	"github.com/GoHyperrr/hyperrr/pkg/eventbus"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

func TestSupportModule(t *testing.T) {
	dbFile := "support_test.db"
	defer os.Remove(dbFile)

	cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
	database, _ := db.Connect(cfg)
	bus := eventbus.NewInMemBus()
	runner := workflow.NewRunner(bus)
	registryStore := workflow.NewRegistry()

	mod := NewModule()
	mod.Init(context.Background(), &registry.Dependencies{DB: database, EventBus: bus, Runner: runner, Registry: registryStore})
	db.Register(mod.Models()...)
	for name, h := range mod.Handlers() { runner.RegisterTask(name, h) }
	database.AutoMigrateAll()

	t.Run("Create Ticket Success", func(t *testing.T) {
		wf := &workflow.Workflow{
			Steps: []workflow.Step{
				{ID: "ticket", Uses: "support.create_ticket"},
			},
		}

		input := map[string]any{
			"customer_id": "cust1",
			"subject":     "Help!",
			"message":     "Need help with my order.",
		}

		res, err := runner.Execute(context.Background(), "t1", wf, input)
		if err != nil {
			t.Fatalf("workflow failed: %v", err)
		}

		resMap := res["ticket"].(map[string]any)
		ticket := resMap["ticket"].(*Ticket)
		if ticket.Status != TicketOpen || len(ticket.Messages) != 1 {
			t.Error("ticket creation failed")
		}
	})

	t.Run("AI Dispatch Response", func(t *testing.T) {
		tkt := &Ticket{ID: "tkt2", CustomerID: "cust2", Status: TicketOpen}
		mod.Repo().SaveTicket(context.Background(), tkt)

		// Test normal response
		results := map[string]any{
			"ticket": map[string]any{"ticket": tkt},
			"input":  map[string]any{},
		}

		res, err := mod.DispatchAIResponse(context.Background(), results)
		if err != nil {
			t.Fatalf("DispatchAIResponse failed: %v", err)
		}

		resMap := res.(map[string]any)
		msg := resMap["message"].(*Message)
		if msg.Sender != SenderAI {
			t.Error("expected AI sender")
		}
	})

	t.Run("Handler Error Cases", func(t *testing.T) {
		// CreateTicket - Invalid Input
		_, err := mod.CreateTicket(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }
		_, err = mod.CreateTicket(context.Background(), map[string]any{"wrong": 1})
		if err == nil { t.Error("expected error for missing workflow input") }

		// DispatchAIResponse - Invalid Input
		_, err = mod.DispatchAIResponse(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }
		_, err = mod.DispatchAIResponse(context.Background(), map[string]any{})
		if err == nil { t.Error("expected error for missing result from ticket step") }
		_, err = mod.DispatchAIResponse(context.Background(), map[string]any{"ticket": "not-a-map"})
		if err == nil { t.Error("expected error for invalid result format") }
		_, err = mod.DispatchAIResponse(context.Background(), map[string]any{"ticket": map[string]any{"wrong": 1}})
		if err == nil { t.Error("expected error for missing ticket in map") }

		// DispatchAIResponse - No special setup needed for default
		tkt := &Ticket{ID: "tkt_no_proj", CustomerID: "cust1", Status: TicketOpen}
		res, err := mod.DispatchAIResponse(context.Background(), map[string]any{"ticket": map[string]any{"ticket": tkt}})
		if err != nil { t.Fatalf("failed without projector: %v", err) }
		if res.(map[string]any)["message"].(*Message).Content != "Hello! I am your AI assistant. How can I help you today?" {
			t.Error("expected default AI message")
		}

		// DispatchAIResponse - Database failure
		badMod := NewModule()
		badMod.repo = NewRepository(nil) // Will cause panic on SaveMessage if not careful, but handler calls m.repo.SaveMessage
		// Actually, to avoid panic and get an error, I should use a repo with a closed DB
		dbFile := "support_bad.db"
		defer os.Remove(dbFile)
		badCfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
		badDB, _ := db.Connect(badCfg)
		sqlDB, _ := badDB.DB.DB()
		badMod.repo = NewRepository(badDB)
		sqlDB.Close()
		_, err = badMod.DispatchAIResponse(context.Background(), map[string]any{"ticket": map[string]any{"ticket": tkt}})
		if err == nil { t.Error("expected error on DB failure") }
	})
}

type mockLineage struct {
	name  string
	state string
	err   string
}

func (m *mockLineage) GetID() string         { return "id" }
func (m *mockLineage) GetName() string       { return m.name }
func (m *mockLineage) GetState() string      { return m.state }
func (m *mockLineage) GetError() string      { return m.err }
func (m *mockLineage) GetStartedAt() time.Time { return time.Now() }
func (m *mockLineage) GetEndedAt() *time.Time  { return nil }

func TestSupportRepository(t *testing.T) {
	dbFile := "support_repo_test.db"
	defer os.Remove(dbFile)
	cfg := &config.Config{DBDriver: "sqlite", DBDSN: dbFile}
	database, _ := db.Connect(cfg)
	
	repo := NewRepository(database)
	database.AutoMigrate(&Ticket{}, &Message{})

	t.Run("CRUD", func(t *testing.T) {
		tkt := &Ticket{ID: "t1", CustomerID: "c1", Status: TicketOpen, Subject: "S"}
		err := repo.SaveTicket(context.Background(), tkt)
		if err != nil { t.Error(err) }

		msg := &Message{ID: "m1", TicketID: "t1", Content: "C", Sender: SenderHuman}
		repo.SaveMessage(context.Background(), msg)

		got, _ := repo.GetTicketByID(context.Background(), "t1")
		if len(got.Messages) != 1 { t.Error("Preload messages failed") }

		list, _ := repo.ListTicketsByCustomerID(context.Background(), "c1")
		if len(list) != 1 { t.Error("ListTicketsByCustomerID failed") }
	})
}
