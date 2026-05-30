package workflow

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type mockNATSStoreKeyValueEntry struct {
	jetstream.KeyValueEntry
	key string
	val []byte
	rev uint64
}

func (e *mockNATSStoreKeyValueEntry) Key() string      { return e.key }
func (e *mockNATSStoreKeyValueEntry) Value() []byte    { return e.val }
func (e *mockNATSStoreKeyValueEntry) Revision() uint64 { return e.rev }
func (e *mockNATSStoreKeyValueEntry) Created() time.Time { return time.Now() }

type mockNATSStoreKeyValue struct {
	jetstream.KeyValue
	store     map[string][]byte
	revs      map[string]uint64
	getErr    error
	createErr error
	updateErr error
	keysErr   error
}

func (m *mockNATSStoreKeyValue) Get(ctx context.Context, key string) (jetstream.KeyValueEntry, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	val, ok := m.store[key]
	if !ok {
		return nil, jetstream.ErrKeyNotFound
	}
	return &mockNATSStoreKeyValueEntry{key: key, val: val, rev: m.revs[key]}, nil
}

func (m *mockNATSStoreKeyValue) Create(ctx context.Context, key string, value []byte, opts ...jetstream.KVCreateOpt) (uint64, error) {
	if m.createErr != nil {
		return 0, m.createErr
	}
	if _, exists := m.store[key]; exists {
		return 0, jetstream.ErrKeyExists
	}
	m.revs[key]++
	m.store[key] = value
	return m.revs[key], nil
}

func (m *mockNATSStoreKeyValue) Update(ctx context.Context, key string, value []byte, revision uint64) (uint64, error) {
	if m.updateErr != nil {
		return 0, m.updateErr
	}
	currentRev, exists := m.revs[key]
	if !exists || currentRev != revision {
		return 0, errors.New("revision mismatch")
	}
	m.revs[key]++
	m.store[key] = value
	return m.revs[key], nil
}

func (m *mockNATSStoreKeyValue) Keys(ctx context.Context, opts ...jetstream.WatchOpt) ([]string, error) {
	if m.keysErr != nil {
		return nil, m.keysErr
	}
	var res []string
	for k := range m.store {
		res = append(res, k)
	}
	return res, nil
}

func TestNATSStore(t *testing.T) {
	storeMap := make(map[string][]byte)
	revs := make(map[string]uint64)
	kv := &mockNATSStoreKeyValue{store: storeMap, revs: revs}

	store := &NATSStore{kv: kv}
	ctx := context.Background()
	execID := "nats-exec-id"

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
		t.Errorf("input mismatch")
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
		_, err := store.GetState(ctx, "non-existent")
		if err == nil {
			t.Error("expected error for missing state")
		}
	})

	t.Run("SaveState Get error", func(t *testing.T) {
		expectedErr := errors.New("mock get error")
		kv.getErr = expectedErr
		defer func() { kv.getErr = nil }()

		err := store.SaveState(ctx, execID, "step-2", StateRunning)
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected mock get error, got %v", err)
		}
	})

	t.Run("SaveState Create error", func(t *testing.T) {
		expectedErr := errors.New("mock create error")
		kv.createErr = expectedErr
		defer func() { kv.createErr = nil }()

		err := store.SaveState(ctx, "fresh-key", "step-2", StateRunning)
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected mock create error, got %v", err)
		}
	})

	t.Run("SaveState Update error", func(t *testing.T) {
		expectedErr := errors.New("mock update error")
		kv.updateErr = expectedErr
		defer func() { kv.updateErr = nil }()

		err := store.SaveState(ctx, execID, "step-1", StateRunning)
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected mock update error, got %v", err)
		}
	})

	t.Run("SaveState Invalid JSON unmarshal", func(t *testing.T) {
		kv.store["wf.bad-json"] = []byte("not-valid-json")
		kv.revs["wf.bad-json"] = 1

		err := store.SaveState(ctx, "bad-json", "step-1", StateRunning)
		if err == nil {
			t.Error("expected JSON unmarshal error")
		}
	})

	t.Run("GetState Get error", func(t *testing.T) {
		expectedErr := errors.New("mock get error")
		kv.getErr = expectedErr
		defer func() { kv.getErr = nil }()

		_, err := store.GetState(ctx, execID)
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected mock get error, got %v", err)
		}
	})

	t.Run("GetState Invalid JSON", func(t *testing.T) {
		kv.store["wf.bad-json-get"] = []byte("not-valid-json")
		_, err := store.GetState(ctx, "bad-json-get")
		if err == nil {
			t.Error("expected JSON unmarshal error")
		}
	})

	t.Run("GetInput Get error", func(t *testing.T) {
		expectedErr := errors.New("mock get error")
		kv.getErr = expectedErr
		defer func() { kv.getErr = nil }()

		_, err := store.GetInput(ctx, execID)
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected mock get error, got %v", err)
		}
	})

	t.Run("GetInput Invalid JSON", func(t *testing.T) {
		kv.store["wf.bad-json-input"] = []byte("not-valid-json")
		_, err := store.GetInput(ctx, "bad-json-input")
		if err == nil {
			t.Error("expected JSON unmarshal error")
		}
	})

	t.Run("GetStepOutput Get error", func(t *testing.T) {
		expectedErr := errors.New("mock get error")
		kv.getErr = expectedErr
		defer func() { kv.getErr = nil }()

		_, err := store.GetStepOutput(ctx, execID, "step-1")
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected mock get error, got %v", err)
		}
	})

	t.Run("GetStepOutput Invalid JSON", func(t *testing.T) {
		kv.store["wf.bad-json-output"] = []byte("not-valid-json")
		_, err := store.GetStepOutput(ctx, "bad-json-output", "step-1")
		if err == nil {
			t.Error("expected JSON unmarshal error")
		}
	})

	t.Run("ListExecutions Keys error", func(t *testing.T) {
		expectedErr := errors.New("mock keys error")
		kv.keysErr = expectedErr
		defer func() { kv.keysErr = nil }()

		_, err := store.ListExecutions(ctx, StateRunning)
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected mock keys error, got %v", err)
		}
	})

	t.Run("ListExecutions Get error", func(t *testing.T) {
		expectedErr := errors.New("mock get error")
		kv.getErr = expectedErr
		defer func() { kv.getErr = nil }()

		ids, err := store.ListExecutions(ctx, StateRunning)
		if err != nil {
			t.Errorf("expected no error from ListExecutions when Get fails, got %v", err)
		}
		if len(ids) != 0 {
			t.Errorf("expected 0 ids, got %d", len(ids))
		}
	})

	t.Run("ListExecutions Invalid JSON", func(t *testing.T) {
		kv.store["wf.bad-json-list"] = []byte("not-valid-json")
		defer delete(kv.store, "wf.bad-json-list")
		ids, err := store.ListExecutions(ctx, StateRunning)
		if err != nil {
			t.Errorf("expected no error from ListExecutions when unmarshal fails, got %v", err)
		}
		for _, id := range ids {
			if id == "bad-json-list" {
				t.Errorf("unexpected bad-json-list id returned")
			}
		}
	})

	t.Run("RecordEventEmitted Get error", func(t *testing.T) {
		expectedErr := errors.New("mock get error")
		kv.getErr = expectedErr
		defer func() { kv.getErr = nil }()

		err := store.RecordEventEmitted(ctx, execID, "event-B")
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected mock get error, got %v", err)
		}
	})

	t.Run("IsEventEmitted Get error", func(t *testing.T) {
		expectedErr := errors.New("mock get error")
		kv.getErr = expectedErr
		defer func() { kv.getErr = nil }()

		_, err := store.IsEventEmitted(ctx, execID, "event-B")
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected mock get error, got %v", err)
		}
	})

	t.Run("IsEventEmitted Invalid JSON", func(t *testing.T) {
		kv.store["wf.bad-json-emitted"] = []byte("not-valid-json")
		_, err := store.IsEventEmitted(ctx, "bad-json-emitted", "event-B")
		if err == nil {
			t.Error("expected JSON unmarshal error")
		}
	})

	t.Run("SaveInput update existing", func(t *testing.T) {
		// SaveInput on an already-initialized exec: exercises the update (non-create) path
		newInput := []byte(`{"userId":"456"}`)
		err := store.SaveInput(ctx, execID, newInput)
		if err != nil {
			t.Fatalf("SaveInput update failed: %v", err)
		}
		got, err := store.GetInput(ctx, execID)
		if err != nil {
			t.Fatalf("GetInput failed: %v", err)
		}
		if string(got) != string(newInput) {
			t.Errorf("expected updated input, got %s", got)
		}
	})

	t.Run("SaveInput Get error", func(t *testing.T) {
		expectedErr := errors.New("mock get error")
		kv.getErr = expectedErr
		defer func() { kv.getErr = nil }()

		err := store.SaveInput(ctx, execID, []byte(`{}`))
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected mock get error, got %v", err)
		}
	})

	t.Run("SaveInput Create error", func(t *testing.T) {
		expectedErr := errors.New("mock create error")
		kv.createErr = expectedErr
		defer func() { kv.createErr = nil }()

		err := store.SaveInput(ctx, "fresh-input-key", []byte(`{}`))
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected mock create error, got %v", err)
		}
	})

	t.Run("SaveInput Invalid JSON unmarshal", func(t *testing.T) {
		kv.store["wf.bad-json-input-save"] = []byte("not-valid-json")
		kv.revs["wf.bad-json-input-save"] = 1

		err := store.SaveInput(ctx, "bad-json-input-save", []byte(`{}`))
		if err == nil {
			t.Error("expected JSON unmarshal error")
		}
	})

	t.Run("SaveInput Update error", func(t *testing.T) {
		expectedErr := errors.New("mock update error")
		kv.updateErr = expectedErr
		defer func() { kv.updateErr = nil }()

		err := store.SaveInput(ctx, execID, []byte(`{"new":"data"}`))
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected mock update error, got %v", err)
		}
	})

	t.Run("SaveStepOutput Get error", func(t *testing.T) {
		expectedErr := errors.New("mock get error")
		kv.getErr = expectedErr
		defer func() { kv.getErr = nil }()

		err := store.SaveStepOutput(ctx, execID, "step-1", []byte(`{}`))
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected mock get error, got %v", err)
		}
	})

	t.Run("SaveStepOutput Update error", func(t *testing.T) {
		expectedErr := errors.New("mock update error")
		kv.updateErr = expectedErr
		defer func() { kv.updateErr = nil }()

		err := store.SaveStepOutput(ctx, execID, "step-1", []byte(`{}`))
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected mock update error, got %v", err)
		}
	})

	t.Run("SaveStepOutput Invalid JSON unmarshal", func(t *testing.T) {
		kv.store["wf.bad-json-output-save"] = []byte("not-valid-json")
		kv.revs["wf.bad-json-output-save"] = 1

		err := store.SaveStepOutput(ctx, "bad-json-output-save", "step-1", []byte(`{}`))
		if err == nil {
			t.Error("expected JSON unmarshal error")
		}
	})

	t.Run("RecordEventEmitted Update error", func(t *testing.T) {
		expectedErr := errors.New("mock update error")
		kv.updateErr = expectedErr
		defer func() { kv.updateErr = nil }()

		err := store.RecordEventEmitted(ctx, execID, "event-C")
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected mock update error, got %v", err)
		}
	})

	t.Run("RecordEventEmitted Invalid JSON unmarshal", func(t *testing.T) {
		kv.store["wf.bad-json-record"] = []byte("not-valid-json")
		kv.revs["wf.bad-json-record"] = 1

		err := store.RecordEventEmitted(ctx, "bad-json-record", "event-C")
		if err == nil {
			t.Error("expected JSON unmarshal error")
		}
	})

	t.Run("IsEventEmitted not found", func(t *testing.T) {
		emitted, err := store.IsEventEmitted(ctx, "non-existent-exec", "event-B")
		if err != nil {
			t.Errorf("expected nil error for missing key, got %v", err)
		}
		if emitted {
			t.Error("expected false for non-existent key")
		}
	})

	t.Run("IsEventEmitted nil EmittedEvents map", func(t *testing.T) {
		// Create a valid entry with no emitted events
		wfState := natsWorkflowState{Steps: make(map[string]string)}
		data, _ := json.Marshal(wfState)
		kv.store["wf.no-events"] = data
		kv.revs["wf.no-events"] = 1

		emitted, err := store.IsEventEmitted(ctx, "no-events", "event-B")
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
		if emitted {
			t.Error("expected false for nil EmittedEvents map")
		}
	})

	t.Run("GetStepOutput nil Outputs map", func(t *testing.T) {
		// Create entry with no outputs
		wfState := natsWorkflowState{Steps: make(map[string]string)}
		data, _ := json.Marshal(wfState)
		kv.store["wf.no-outputs"] = data
		kv.revs["wf.no-outputs"] = 1

		_, err := store.GetStepOutput(ctx, "no-outputs", "step-1")
		if err == nil {
			t.Error("expected error for nil outputs map")
		}
	})

	t.Run("GetInput nil input in state", func(t *testing.T) {
		wfState := natsWorkflowState{Steps: make(map[string]string)}
		data, _ := json.Marshal(wfState)
		kv.store["wf.no-input"] = data
		kv.revs["wf.no-input"] = 1

		_, err := store.GetInput(ctx, "no-input")
		if err == nil {
			t.Error("expected error for nil input in state")
		}
	})

	t.Run("SaveState overall state path", func(t *testing.T) {
		err := store.SaveState(ctx, execID, "", StateCompleted)
		if err != nil {
			t.Fatalf("SaveState overall state failed: %v", err)
		}
	})

	t.Run("ListExecutions no wf prefix skip", func(t *testing.T) {
		// Add a key without wf. prefix
		kv.store["other.key"] = []byte(`{"overall_state":"RUNNING"}`)
		kv.revs["other.key"] = 1
		defer delete(kv.store, "other.key")
		defer delete(kv.revs, "other.key")

		ids, err := store.ListExecutions(ctx, StateRunning)
		if err != nil {
			t.Fatalf("ListExecutions failed: %v", err)
		}
		for _, id := range ids {
			if id == "key" {
				t.Error("should not return non-wf prefixed keys")
			}
		}
	})

	t.Run("ListExecutions ErrNoKeysFound", func(t *testing.T) {
		kv.keysErr = jetstream.ErrNoKeysFound
		defer func() { kv.keysErr = nil }()

		ids, err := store.ListExecutions(ctx, StateRunning)
		if err != nil {
			t.Errorf("expected nil error for ErrNoKeysFound, got %v", err)
		}
		if ids != nil {
			t.Errorf("expected nil ids, got %v", ids)
		}
	})
}

func TestNewNATSStore(t *testing.T) {
	t.Run("Nil connection", func(t *testing.T) {
		_, err := NewNATSStore(context.Background(), nil, "testbucket")
		if err == nil {
			t.Error("expected error with nil connection")
		}
	})

	t.Run("Dummy NATS connection fail CreateOrUpdateKeyValue", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to listen: %v", err)
		}
		defer ln.Close()

		go func() {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			defer conn.Close()
			_, _ = conn.Write([]byte("INFO {\"server_id\":\"mock\",\"max_payload\":1048576}\r\n"))
			reader := bufio.NewReader(conn)
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if strings.Contains(line, "PING") {
					_, _ = conn.Write([]byte("PONG\r\n"))
				}
			}
		}()

		addr := "nats://" + ln.Addr().String()
		nc, err := nats.Connect(addr, nats.Timeout(2*time.Second))
		if err != nil {
			t.Fatalf("failed to connect to mock: %v", err)
		}
		defer nc.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		_, err = NewNATSStore(ctx, nc, "testbucket")
		if err == nil {
			t.Error("expected error creating KeyValue bucket on mock server")
		}
	})
}
