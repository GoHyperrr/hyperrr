package eventbus

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

type mockNatsServer struct {
	listener net.Listener
	mu       sync.Mutex
	conns    []net.Conn
	closed   bool
}

func startMockNatsServer(t *testing.T) *mockNatsServer {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock NATS server: %v", err)
	}
	s := &mockNatsServer{
		listener: ln,
		conns:    make([]net.Conn, 0),
	}
	go s.acceptConnections()
	return s
}

func (s *mockNatsServer) Addr() string {
	return "nats://" + s.listener.Addr().String()
}

func (s *mockNatsServer) Close() {
	s.mu.Lock()
	s.closed = true
	s.listener.Close()
	for _, c := range s.conns {
		c.Close()
	}
	s.mu.Unlock()
}

func (s *mockNatsServer) acceptConnections() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		if s.closed {
			conn.Close()
			s.mu.Unlock()
			return
		}
		s.conns = append(s.conns, conn)
		s.mu.Unlock()
		go s.handleConnection(conn)
	}
}

func (s *mockNatsServer) handleConnection(conn net.Conn) {
	defer conn.Close()
	writer := bufio.NewWriter(conn)
	reader := bufio.NewReader(conn)

	// Send INFO
	_, _ = writer.WriteString("INFO {\"server_id\":\"mock\",\"max_payload\":1048576}\r\n")
	_ = writer.Flush()

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		parts := strings.Split(line, " ")
		cmd := strings.ToUpper(parts[0])

		switch cmd {
		case "CONNECT":
			// accepted
		case "PING":
			_, _ = writer.WriteString("PONG\r\n")
			_ = writer.Flush()
		case "PUB":
			// Read the payload (next line)
			if len(parts) >= 3 {
				_, _ = reader.ReadString('\n')
			}
		case "SUB":
			// accepted
		case "UNSUB":
			// accepted
		}
	}
}

func (s *mockNatsServer) broadcastMessage(t *testing.T, subject string, sid string, payload []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	msg := fmt.Sprintf("MSG %s %s %d\r\n%s\r\n", subject, sid, len(payload), string(payload))
	for _, c := range s.conns {
		_, err := c.Write([]byte(msg))
		if err != nil {
			t.Logf("failed to write to connection: %v", err)
		}
	}
}

func TestNATSBus_LifecycleAndOperations(t *testing.T) {
	server := startMockNatsServer(t)
	defer server.Close()

	bus, err := NewNATSBus(server.Addr())
	if err != nil {
		t.Fatalf("NewNATSBus failed: %v", err)
	}
	defer bus.Close()

	// 1. SetContext
	ctx := context.Background()
	bus.SetContext(ctx)

	// 2. Subscribe and wait for event
	var wg sync.WaitGroup
	wg.Add(1)

	var received Event
	unsub, err := bus.Subscribe("test", "event", func(ctx context.Context, ev Event) error {
		received = ev
		wg.Done()
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Wait for connection subscription registry
	time.Sleep(50 * time.Millisecond)

	// Broadcast from mock server (subject is "test.event")
	sentEvent := Event{
		ID:         "evt-123",
		Namespace:  "test",
		Type:       "event",
		Payload:    map[string]any{"data": "hello"},
		OccurredAt: time.Now(),
	}
	payloadBytes, _ := json.Marshal(sentEvent)
	
	// NATS subscription ID is typically "1" for the first subscription
	server.broadcastMessage(t, "test.event", "1", payloadBytes)

	// Wait for handler to receive message
	c := make(chan struct{})
	go func() {
		wg.Wait()
		close(c)
	}()

	select {
	case <-c:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for event handler")
	}

	if received.ID != sentEvent.ID {
		t.Errorf("expected received event ID %q, got %q", sentEvent.ID, received.ID)
	}

	// 3. Publish
	err = bus.Publish(ctx, sentEvent)
	if err != nil {
		t.Errorf("Publish failed: %v", err)
	}

	// 4. Conn getter
	if bus.Conn() == nil {
		t.Error("expected underlying nats.Conn to not be nil")
	}

	// 5. Unsubscribe
	unsub()
}

func TestNATSBus_NilConnectionErrors(t *testing.T) {
	bus := &NATSBus{conn: nil}
	ctx := context.Background()

	err := bus.Publish(ctx, Event{})
	if err == nil {
		t.Error("expected error publishing on nil connection")
	}

	_, err = bus.Subscribe("test", "event", func(ctx context.Context, ev Event) error { return nil })
	if err == nil {
		t.Error("expected error subscribing on nil connection")
	}
}

func TestNATSBus_ErrorCases(t *testing.T) {
	// 1. Invalid URL for NewNATSBus (forces nats.Connect error)
	_, err := NewNATSBus("nats://127.0.0.1:23")
	if err == nil {
		t.Error("expected error with invalid NATS URL")
	}

	// 2. Publish and Subscribe errors on closed connection
	server := startMockNatsServer(t)
	defer server.Close()

	bus, _ := NewNATSBus(server.Addr())
	_ = bus.Close() // Close immediately

	err = bus.Publish(context.Background(), Event{})
	if err == nil {
		t.Error("expected error publishing on closed bus")
	}

	_, err = bus.Subscribe("test", "event", func(ctx context.Context, ev Event) error { return nil })
	if err == nil {
		t.Error("expected error subscribing on closed bus")
	}

	// 3. Double Close
	err = bus.Close()
	if err != nil {
		t.Errorf("expected nil error on double close, got %v", err)
	}
}

func TestNATSBus_AdditionalCoverage(t *testing.T) {
	server := startMockNatsServer(t)
	defer server.Close()

	bus, err := NewNATSBus(server.Addr())
	if err != nil {
		t.Fatalf("NewNATSBus failed: %v", err)
	}
	defer bus.Close()

	ctx := context.Background()

	t.Run("Publish json marshal error", func(t *testing.T) {
		event := Event{
			Namespace: "marshal",
			Type:      "error",
			Payload:   map[string]any{"data": make(chan int)}, // Unmarshalable
		}
		err := bus.Publish(ctx, event)
		if err == nil {
			t.Error("expected marshal error, got nil")
		}
	})

	t.Run("Subscribe connection error", func(t *testing.T) {
		bus.conn.Close()
		_, err := bus.Subscribe("test", "sub.err", func(ctx context.Context, ev Event) error { return nil })
		if err == nil {
			t.Error("expected subscribe error on closed connection, got nil")
		}
	})
}

func TestNATSBus_HandlerErrors(t *testing.T) {
	server := startMockNatsServer(t)
	defer server.Close()

	bus, err := NewNATSBus(server.Addr())
	if err != nil {
		t.Fatalf("NewNATSBus failed: %v", err)
	}
	defer bus.Close()

	ctx := context.Background()
	bus.SetContext(ctx)

	_, _ = bus.Subscribe("bad", "json", func(ctx context.Context, ev Event) error { return nil })
	time.Sleep(20 * time.Millisecond)
	server.broadcastMessage(t, "bad.json", "1", []byte("invalid-json"))
	time.Sleep(20 * time.Millisecond)

	_, _ = bus.Subscribe("handler", "err", func(ctx context.Context, ev Event) error {
		return errors.New("handler error")
	})
	time.Sleep(20 * time.Millisecond)
	evt := Event{ID: "evt-999", Namespace: "handler", Type: "err"}
	evtBytes, _ := json.Marshal(evt)
	server.broadcastMessage(t, "handler.err", "2", evtBytes)
	time.Sleep(20 * time.Millisecond)
}
