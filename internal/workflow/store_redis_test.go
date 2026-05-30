package workflow

import (
	"bufio"
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type mockRedisServer struct {
	listener net.Listener
	mu       sync.Mutex
	conns    []net.Conn
	closed   bool
}

func startMockRedisServer(t *testing.T) *mockRedisServer {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock Redis server: %v", err)
	}
	s := &mockRedisServer{
		listener: ln,
		conns:    make([]net.Conn, 0),
	}
	go s.acceptConnections()
	return s
}

func (s *mockRedisServer) Addr() string {
	return s.listener.Addr().String()
}

func (s *mockRedisServer) Close() {
	s.mu.Lock()
	s.closed = true
	s.listener.Close()
	for _, c := range s.conns {
		c.Close()
	}
	s.mu.Unlock()
}

func (s *mockRedisServer) acceptConnections() {
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

func (s *mockRedisServer) handleConnection(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		_, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		_, _ = writer.WriteString("+OK\r\n")
		_ = writer.Flush()
	}
}

type mockStoreRedisHook struct {
	store  map[string]any
	cmdErr error
}

func (h *mockStoreRedisHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h *mockStoreRedisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook { return next }

func (h *mockStoreRedisHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if h.cmdErr != nil {
			return h.cmdErr
		}
		name := strings.ToLower(cmd.Name())
		args := cmd.Args()

		if name == "set" {
			key := args[1].(string)
			val := args[2]
			h.store[key] = val
			if statusCmd, ok := cmd.(*redis.StatusCmd); ok {
				statusCmd.SetVal("OK")
			}
			return nil
		} else if name == "get" {
			key := args[1].(string)
			val, ok := h.store[key]
			if !ok {
				return redis.Nil
			}
			if stringCmd, ok := cmd.(*redis.StringCmd); ok {
				switch v := val.(type) {
				case string:
					stringCmd.SetVal(v)
				case []byte:
					stringCmd.SetVal(string(v))
				}
			}
			return nil
		} else if name == "hset" {
			key := args[1].(string)
			field := args[2].(string)
			val := args[3]
			
			m, ok := h.store[key].(map[string]string)
			if !ok {
				m = make(map[string]string)
				h.store[key] = m
			}
			switch v := val.(type) {
			case string:
				m[field] = v
			case []byte:
				m[field] = string(v)
			}
			if intCmd, ok := cmd.(*redis.IntCmd); ok {
				intCmd.SetVal(1)
			}
			return nil
		} else if name == "hget" {
			key := args[1].(string)
			field := args[2].(string)
			m, ok := h.store[key].(map[string]string)
			if !ok {
				return redis.Nil
			}
			val, ok := m[field]
			if !ok {
				return redis.Nil
			}
			if stringCmd, ok := cmd.(*redis.StringCmd); ok {
				stringCmd.SetVal(val)
			}
			return nil
		} else if name == "hgetall" {
			key := args[1].(string)
			m, ok := h.store[key].(map[string]string)
			if !ok {
				m = make(map[string]string)
			}
			if mapCmd, ok := cmd.(*redis.MapStringStringCmd); ok {
				mapCmd.SetVal(m)
			}
			return nil
		} else if name == "hexists" {
			key := args[1].(string)
			field := args[2].(string)
			m, ok := h.store[key].(map[string]string)
			exists := ok && m != nil && m[field] != ""
			if boolCmd, ok := cmd.(*redis.BoolCmd); ok {
				boolCmd.SetVal(exists)
			}
			return nil
		} else if name == "expire" {
			if intCmd, ok := cmd.(*redis.IntCmd); ok {
				intCmd.SetVal(1)
			}
			return nil
		} else if name == "scan" {
			var keys []string
			for k, v := range h.store {
				if strings.HasPrefix(k, "wf:") && strings.HasSuffix(k, ":overall") {
					if s, ok := v.(string); ok && s == "RUNNING" {
						keys = append(keys, k)
					}
				}
			}
			if scanCmd, ok := cmd.(*redis.ScanCmd); ok {
				scanCmd.SetVal(keys, 0)
			}
			return nil
		}

		return nil
	}
}

func TestRedisStore(t *testing.T) {
	server := startMockRedisServer(t)
	defer server.Close()

	client := redis.NewClient(&redis.Options{
		Addr: server.Addr(),
	})
	hook := &mockStoreRedisHook{store: make(map[string]any)}
	client.AddHook(hook)

	store := NewRedisStore(client)
	ctx := context.Background()
	execID := "redis-exec-id"

	// 1. InitializeExecution
	input := []byte(`{"userId":"123"}`)
	err := store.InitializeExecution(ctx, execID, input)
	if err != nil {
		t.Fatalf("InitializeExecution failed: %v", err)
	}

	// 2. SaveInput & GetInput
	err = store.SaveInput(ctx, execID, input)
	if err != nil {
		t.Fatalf("SaveInput failed: %v", err)
	}
	gotInput, err := store.GetInput(ctx, execID)
	if err != nil {
		t.Fatalf("GetInput failed: %v", err)
	}
	if string(gotInput) != string(input) {
		t.Errorf("input mismatch: %s vs %s", string(gotInput), string(input))
	}

	// 3. SaveState & GetState
	err = store.SaveState(ctx, execID, "step-1", StateRunning)
	if err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}
	states, err := store.GetState(ctx, execID)
	if err != nil {
		t.Fatalf("GetState failed: %v", err)
	}
	if states["step-1"] != StateRunning {
		t.Errorf("expected step-1 to be RUNNING, got %v", states["step-1"])
	}

	// 4. SaveStepOutput & GetStepOutput
	out := []byte(`{"step-res":"ok"}`)
	err = store.SaveStepOutput(ctx, execID, "step-1", out)
	if err != nil {
		t.Fatalf("SaveStepOutput failed: %v", err)
	}
	gotOut, err := store.GetStepOutput(ctx, execID, "step-1")
	if err != nil {
		t.Fatalf("GetStepOutput failed: %v", err)
	}
	if string(gotOut) != string(out) {
		t.Errorf("output mismatch")
	}

	// 5. ListExecutions
	_ = store.SaveState(ctx, execID, "", StateRunning) // Overall state
	execs, err := store.ListExecutions(ctx, StateRunning)
	if err != nil {
		t.Fatalf("ListExecutions failed: %v", err)
	}
	if len(execs) != 1 || execs[0] != execID {
		t.Errorf("expected ListExecutions to return %q, got %v", execID, execs)
	}

	// 6. RecordEventEmitted & IsEventEmitted
	err = store.RecordEventEmitted(ctx, execID, "event-A")
	if err != nil {
		t.Fatalf("RecordEventEmitted failed: %v", err)
	}
	emitted, err := store.IsEventEmitted(ctx, execID, "event-A")
	if err != nil {
		t.Fatalf("IsEventEmitted failed: %v", err)
	}
	if !emitted {
		t.Error("expected event-A to be emitted")
	}

	// 7. SetTTL
	err = store.SetTTL(ctx, execID, 1*time.Hour)
	if err != nil {
		t.Fatalf("SetTTL failed: %v", err)
	}

	// Test Error paths
	t.Run("GetInput Missing", func(t *testing.T) {
		_, err := store.GetInput(ctx, "non-existent")
		if err == nil {
			t.Error("expected error for missing input")
		}
	})

	t.Run("GetStepOutput Missing", func(t *testing.T) {
		_, err := store.GetStepOutput(ctx, execID, "non-existent")
		if err == nil {
			t.Error("expected error for missing output")
		}
	})

	t.Run("GetState Missing", func(t *testing.T) {
		delete(hook.store, "wf:"+execID+":state")
		_, err := store.GetState(ctx, execID)
		if err == nil {
			t.Error("expected error for missing state")
		}
	})

	t.Run("Redis Error Paths", func(t *testing.T) {
		expectedErr := errors.New("mock redis error")
		hook.cmdErr = expectedErr
		defer func() { hook.cmdErr = nil }()

		if err := store.SaveState(ctx, execID, "step-1", StateRunning); err == nil {
			t.Error("expected error from SaveState")
		}
		if _, err := store.GetState(ctx, execID); err == nil {
			t.Error("expected error from GetState")
		}
		if err := store.InitializeExecution(ctx, "new-id", input); err == nil {
			t.Error("expected error from InitializeExecution")
		}
		if err := store.SaveInput(ctx, execID, input); err == nil {
			t.Error("expected error from SaveInput")
		}
		if _, err := store.GetInput(ctx, execID); err == nil {
			t.Error("expected error from GetInput")
		}
		if err := store.SetTTL(ctx, execID, 1*time.Hour); err == nil {
			t.Error("expected error from SetTTL")
		}
		if err := store.SaveStepOutput(ctx, execID, "step-1", out); err == nil {
			t.Error("expected error from SaveStepOutput")
		}
		if _, err := store.GetStepOutput(ctx, execID, "step-1"); err == nil {
			t.Error("expected error from GetStepOutput")
		}
		if _, err := store.ListExecutions(ctx, StateRunning); err == nil {
			t.Error("expected error from ListExecutions")
		}
		if err := store.RecordEventEmitted(ctx, execID, "evt"); err == nil {
			t.Error("expected error from RecordEventEmitted")
		}
		if _, err := store.IsEventEmitted(ctx, execID, "evt"); err == nil {
			t.Error("expected error from IsEventEmitted")
		}
	})
}
