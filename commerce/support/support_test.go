package support

import (
	"context"
	"os"
	"testing"

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

	mod := NewModule()
	mod.Init(context.Background(), &registry.Dependencies{DB: database, EventBus: bus, Runner: runner})
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

		// Dispatch - Invalid Input
		_, err = mod.DispatchAIResponse(context.Background(), "string")
		if err == nil { t.Error("expected error for invalid input type") }
		_, err = mod.DispatchAIResponse(context.Background(), map[string]any{})
		if err == nil { t.Error("expected error for missing ticket") }
	})
}

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
