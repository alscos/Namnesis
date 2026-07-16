package controlstate

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alscos/Namnesis/internal/stompbox"
)

type protocolFixture struct {
	listener net.Listener

	mu          sync.Mutex
	program     string
	failProgram bool
	counts      map[string]int
}

func newProtocolFixture(t *testing.T) *protocolFixture {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &protocolFixture{
		listener: ln,
		program:  "SetPreset 01_Bassman\r\nEndProgram\r\nOk\r\n",
		counts:   make(map[string]int),
	}
	go f.serve()
	t.Cleanup(func() { _ = ln.Close() })
	return f
}

func (f *protocolFixture) serve() {
	for {
		conn, err := f.listener.Accept()
		if err != nil {
			return
		}
		go f.handle(conn)
	}
}

func (f *protocolFixture) handle(conn net.Conn) {
	defer conn.Close()
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return
	}
	command := strings.TrimSpace(line)

	f.mu.Lock()
	f.counts[command]++
	program := f.program
	failProgram := f.failProgram
	f.mu.Unlock()

	var response string
	switch command {
	case "Dump Config":
		response = "PluginConfig NAM\r\nEndConfig\r\nOk\r\n"
	case "Dump Program":
		if failProgram {
			return
		}
		response = program
	case "List Presets":
		response = "01_Bassman\r\n02_Deluxe\r\nOk\r\n"
	default:
		response = "Ok\r\n"
	}
	_, _ = fmt.Fprint(conn, response)
}

func (f *protocolFixture) setProgram(raw string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.program = raw
}

func (f *protocolFixture) setProgramFailure(fail bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failProgram = fail
}

func (f *protocolFixture) count(command string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.counts[command]
}

func TestSnapshotIsCacheOnlyAndProgramEventsAreAuthoritative(t *testing.T) {
	fixture := newProtocolFixture(t)
	client := stompbox.New(fixture.listener.Addr().String())
	client.DialTimeout = 100 * time.Millisecond
	client.ReadTimeout = 100 * time.Millisecond

	manager := New(client, time.Hour, time.Hour, time.Hour)
	if err := manager.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	if got := fixture.count("Dump Config"); got != 1 {
		t.Fatalf("Dump Config count = %d, want 1", got)
	}
	if got := fixture.count("Dump Program"); got != 1 {
		t.Fatalf("Dump Program count = %d, want 1", got)
	}
	if got := fixture.count("List Presets"); got != 1 {
		t.Fatalf("List Presets count = %d, want 1", got)
	}

	started := time.Now()
	for i := 0; i < 10_000; i++ {
		snapshot := manager.Snapshot()
		if snapshot.Program.Raw == "" {
			t.Fatal("cached program unexpectedly empty")
		}
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("10,000 cache snapshots took %s; snapshots should not perform TCP I/O", elapsed)
	}
	if got := fixture.count("Dump Program"); got != 1 {
		t.Fatalf("cache reads caused %d Dump Program calls, want 1", got)
	}

	events, cancel := manager.Subscribe()
	defer cancel()
	fixture.setProgram("SetPreset 02_Deluxe\r\nEndProgram\r\nOk\r\n")
	if err := manager.RefreshProgramNow("test-change"); err != nil {
		t.Fatalf("refresh program: %v", err)
	}

	select {
	case event := <-events:
		if event.Type != "program" || event.Reason != "test-change" {
			t.Fatalf("unexpected event: %#v", event)
		}
		if !strings.Contains(event.Section.Raw, "02_Deluxe") {
			t.Fatalf("event did not contain refreshed program: %q", event.Section.Raw)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for program event")
	}
}

func TestTransientProgramFailureKeepsLastKnownState(t *testing.T) {
	fixture := newProtocolFixture(t)
	client := stompbox.New(fixture.listener.Addr().String())
	client.DialTimeout = 100 * time.Millisecond
	client.ReadTimeout = 50 * time.Millisecond

	manager := New(client, time.Hour, time.Hour, time.Hour)
	if err := manager.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	before, err := manager.ProgramRaw()
	if err != nil {
		t.Fatalf("program before failure: %v", err)
	}

	fixture.setProgramFailure(true)
	if err := manager.RefreshProgramNow("forced-failure"); err == nil {
		t.Fatal("expected failed program refresh")
	}

	after, err := manager.ProgramRaw()
	if err != nil {
		t.Fatalf("last-known program should remain readable: %v", err)
	}
	if after != before {
		t.Fatalf("last-known program changed after failed refresh\nbefore=%q\nafter=%q", before, after)
	}

	view := manager.Snapshot().Program
	if !view.Stale || view.Error == "" {
		t.Fatalf("failed refresh should expose stale cached state: %#v", view)
	}
}

func TestResolvedProgramMetadataAndRuntimeOverride(t *testing.T) {
	fixture := newProtocolFixture(t)
	client := stompbox.New(fixture.listener.Addr().String())
	client.DialTimeout = 100 * time.Millisecond
	client.ReadTimeout = 100 * time.Millisecond

	manager := New(client, time.Hour, time.Hour, time.Hour)
	manager.SetProgramResolver(func(raw string) map[string]map[string]string {
		if strings.Contains(raw, "01_Bassman") {
			return map[string]map[string]string{"NAM": {"Model": "PresetModel"}}
		}
		return map[string]map[string]string{"NAM": {"Model": "OtherPresetModel"}}
	})
	if err := manager.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	if got := manager.Snapshot().Program.Resolved["NAM"]["Model"]; got != "PresetModel" {
		t.Fatalf("resolved model = %q, want PresetModel", got)
	}

	manager.SetProgramParamOverride("NAM", "Model", "LiveModel")
	if err := manager.RefreshProgramNow("file-param"); err != nil {
		t.Fatalf("refresh after override: %v", err)
	}
	if got := manager.Snapshot().Program.Resolved["NAM"]["Model"]; got != "LiveModel" {
		t.Fatalf("override model = %q, want LiveModel", got)
	}

	fixture.setProgram("SetPreset 02_Deluxe\r\nEndProgram\r\nOk\r\n")
	if err := manager.RefreshProgramNow("preset-change"); err != nil {
		t.Fatalf("refresh changed preset: %v", err)
	}
	if got := manager.Snapshot().Program.Resolved["NAM"]["Model"]; got != "OtherPresetModel" {
		t.Fatalf("override leaked across preset change: got %q", got)
	}
}
