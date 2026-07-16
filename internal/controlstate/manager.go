package controlstate

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alscos/Namnesis/internal/stompbox"
)

type Section struct {
	Raw         string                       `json:"raw,omitempty"`
	Resolved    map[string]map[string]string `json:"resolved,omitempty"`
	Duration    string                       `json:"duration"`
	Error       string                       `json:"error,omitempty"`
	UpdatedAt   time.Time                    `json:"-"`
	AttemptedAt time.Time                    `json:"-"`
}

type SectionView struct {
	Raw          string                       `json:"raw,omitempty"`
	Resolved     map[string]map[string]string `json:"resolved,omitempty"`
	Duration     string                       `json:"duration"`
	Error        string                       `json:"error,omitempty"`
	UpdatedAt    string                       `json:"updatedAt,omitempty"`
	AttemptedAt  string                       `json:"attemptedAt,omitempty"`
	AgeMS        int64                        `json:"ageMs"`
	AttemptAgeMS int64                        `json:"attemptAgeMs"`
	Cached       bool                         `json:"cached"`
	Stale        bool                         `json:"stale"`
}

type MetaView struct {
	Now             string `json:"now"`
	Revision        uint64 `json:"revision"`
	Connected       bool   `json:"connected"`
	LastReason      string `json:"lastReason,omitempty"`
	SubscriberCount int    `json:"subscriberCount"`
}

type SnapshotView struct {
	Meta       MetaView    `json:"meta"`
	DumpConfig SectionView `json:"dumpConfig"`
	Program    SectionView `json:"program"`
	Presets    SectionView `json:"presets"`
}

type Event struct {
	Type     string      `json:"type"`
	Reason   string      `json:"reason,omitempty"`
	Revision uint64      `json:"revision"`
	Section  SectionView `json:"section"`
}

type refreshRequest struct {
	kind   string
	reason string
}

type ProgramResolver func(programRaw string) map[string]map[string]string

type Manager struct {
	sb *stompbox.Client

	programPoll   time.Duration
	configRefresh time.Duration
	presetRefresh time.Duration

	mu             sync.RWMutex
	config         Section
	program        Section
	presets        Section
	lastReason     string
	resolver       ProgramResolver
	overrides      map[string]map[string]string
	overridePreset string

	revision atomic.Uint64

	subsMu sync.Mutex
	subs   map[chan Event]struct{}

	pendingMu sync.Mutex
	pending   map[string]bool
	refreshCh chan refreshRequest
}

func New(
	sb *stompbox.Client,
	programPoll time.Duration,
	configRefresh time.Duration,
	presetRefresh time.Duration,
) *Manager {
	if programPoll <= 0 {
		programPoll = 250 * time.Millisecond
	}
	if configRefresh <= 0 {
		configRefresh = 10 * time.Minute
	}
	if presetRefresh <= 0 {
		presetRefresh = 30 * time.Second
	}

	return &Manager{
		sb:            sb,
		programPoll:   programPoll,
		configRefresh: configRefresh,
		presetRefresh: presetRefresh,
		subs:          make(map[chan Event]struct{}),
		pending:       make(map[string]bool),
		refreshCh:     make(chan refreshRequest, 16),
		overrides:     make(map[string]map[string]string),
	}
}

func (m *Manager) SetProgramResolver(resolver ProgramResolver) {
	m.mu.Lock()
	m.resolver = resolver
	m.mu.Unlock()
}

// SetProgramParamOverride preserves a successfully written file parameter when
// Stompbox omits it from Dump Program. Overrides are scoped to the active preset
// and are discarded automatically as soon as the preset changes.
func (m *Manager) SetProgramParamOverride(plugin, param, value string) {
	plugin = strings.TrimSpace(plugin)
	param = strings.TrimSpace(param)
	value = strings.TrimSpace(value)
	if plugin == "" || param == "" || value == "" {
		return
	}

	m.mu.Lock()
	preset := presetFromProgram(m.program.Raw)
	if preset != m.overridePreset {
		m.overrides = make(map[string]map[string]string)
		m.overridePreset = preset
	}
	if m.overrides[plugin] == nil {
		m.overrides[plugin] = make(map[string]string)
	}
	m.overrides[plugin][param] = value
	m.mu.Unlock()
}

func (m *Manager) Bootstrap(ctx context.Context) error {
	var errs []error
	if err := m.refreshConfig("bootstrap"); err != nil {
		errs = append(errs, fmt.Errorf("config: %w", err))
	}
	if err := m.refreshProgram("bootstrap"); err != nil {
		errs = append(errs, fmt.Errorf("program: %w", err))
	}
	if err := m.refreshPresets("bootstrap"); err != nil {
		errs = append(errs, fmt.Errorf("presets: %w", err))
	}

	select {
	case <-ctx.Done():
		errs = append(errs, ctx.Err())
	default:
	}
	return errors.Join(errs...)
}

func (m *Manager) Run(ctx context.Context) {
	programTicker := time.NewTicker(m.programPoll)
	configTicker := time.NewTicker(m.configRefresh)
	presetTicker := time.NewTicker(m.presetRefresh)
	defer programTicker.Stop()
	defer configTicker.Stop()
	defer presetTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.closeSubscribers()
			return
		case <-programTicker.C:
			if m.refreshDue("program", m.programPoll) {
				_ = m.refreshProgram("poll")
			}
		case <-configTicker.C:
			if m.refreshDue("config", m.configRefresh) {
				_ = m.refreshConfig("periodic")
			}
		case <-presetTicker.C:
			if m.refreshDue("presets", m.presetRefresh) {
				_ = m.refreshPresets("periodic")
			}
		case req := <-m.refreshCh:
			m.clearPending(req.kind)
			switch req.kind {
			case "program":
				_ = m.refreshProgram(req.reason)
			case "presets":
				_ = m.refreshPresets(req.reason)
			case "config":
				_ = m.refreshConfig(req.reason)
			case "all":
				_ = m.refreshConfig(req.reason)
				_ = m.refreshProgram(req.reason)
				_ = m.refreshPresets(req.reason)
			}
		}
	}
}

func (m *Manager) TriggerProgram(reason string) {
	m.trigger("program", reason)
}

func (m *Manager) TriggerPresets(reason string) {
	m.trigger("presets", reason)
}

func (m *Manager) TriggerConfig(reason string) {
	m.trigger("config", reason)
}

func (m *Manager) TriggerAll(reason string) {
	m.trigger("all", reason)
}

func (m *Manager) trigger(kind, reason string) {
	m.pendingMu.Lock()
	if m.pending[kind] {
		m.pendingMu.Unlock()
		return
	}
	m.pending[kind] = true
	m.pendingMu.Unlock()

	select {
	case m.refreshCh <- refreshRequest{kind: kind, reason: reason}:
	default:
		m.clearPending(kind)
		// The periodic observer still converges to authoritative DSP state.
	}
}

func (m *Manager) clearPending(kind string) {
	m.pendingMu.Lock()
	delete(m.pending, kind)
	m.pendingMu.Unlock()
}

func (m *Manager) refreshDue(kind string, interval time.Duration) bool {
	m.mu.RLock()
	var attempted time.Time
	switch kind {
	case "config":
		attempted = m.config.AttemptedAt
	case "program":
		attempted = m.program.AttemptedAt
	case "presets":
		attempted = m.presets.AttemptedAt
	}
	m.mu.RUnlock()
	return attempted.IsZero() || time.Since(attempted) >= interval
}

func (m *Manager) RefreshProgramNow(reason string) error {
	return m.refreshProgram(reason)
}

func (m *Manager) RefreshPresetsNow(reason string) error {
	return m.refreshPresets(reason)
}

func (m *Manager) RefreshConfigNow(reason string) error {
	return m.refreshConfig(reason)
}

func (m *Manager) RefreshAllNow(reason string) error {
	var errs []error
	if err := m.refreshConfig(reason); err != nil {
		errs = append(errs, err)
	}
	if err := m.refreshProgram(reason); err != nil {
		errs = append(errs, err)
	}
	if err := m.refreshPresets(reason); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (m *Manager) refreshConfig(reason string) error {
	started := time.Now()
	raw, err := m.sb.DumpConfig()
	return m.store("config", raw, nil, err, time.Since(started), reason)
}

func (m *Manager) refreshProgram(reason string) error {
	started := time.Now()
	raw, err := m.sb.DumpProgram()
	if err != nil {
		return m.store("program", raw, nil, err, time.Since(started), reason)
	}
	resolved := m.resolveProgram(raw)
	return m.store("program", raw, resolved, nil, time.Since(started), reason)
}

func (m *Manager) refreshPresets(reason string) error {
	started := time.Now()
	raw, err := m.sb.ListPresets()
	return m.store("presets", raw, nil, err, time.Since(started), reason)
}

func (m *Manager) store(kind, raw string, resolved map[string]map[string]string, err error, duration time.Duration, reason string) error {
	now := time.Now()

	m.mu.Lock()
	var previous Section
	switch kind {
	case "config":
		previous = m.config
	case "program":
		previous = m.program
	case "presets":
		previous = m.presets
	}

	// Stale-while-error: a transient TCP failure must not blank the control
	// surface. Keep the last authoritative payload, expose the failed attempt,
	// and replace the payload only after a successful Stompbox response.
	newSection := previous
	newSection.Duration = duration.String()
	newSection.AttemptedAt = now
	if err == nil {
		newSection.Raw = raw
		newSection.Resolved = cloneResolved(resolved)
		newSection.Error = ""
		newSection.UpdatedAt = now
	} else {
		newSection.Error = err.Error()
	}

	switch kind {
	case "config":
		m.config = newSection
	case "program":
		m.program = newSection
	case "presets":
		m.presets = newSection
	}
	m.lastReason = reason
	changed := previous.Raw != newSection.Raw || previous.Error != newSection.Error || !resolvedEqual(previous.Resolved, newSection.Resolved)
	m.mu.Unlock()

	if changed {
		rev := m.revision.Add(1)
		m.broadcast(Event{
			Type:     kind,
			Reason:   reason,
			Revision: rev,
			Section:  sectionView(newSection, now),
		})
	}
	return err
}

func (m *Manager) Snapshot() SnapshotView {
	now := time.Now()
	m.mu.RLock()
	config := m.config
	program := m.program
	presets := m.presets
	lastReason := m.lastReason
	m.mu.RUnlock()

	m.subsMu.Lock()
	subscriberCount := len(m.subs)
	m.subsMu.Unlock()

	connected := program.Error == "" && program.Raw != ""
	return SnapshotView{
		Meta: MetaView{
			Now:             now.Format(time.RFC3339Nano),
			Revision:        m.revision.Load(),
			Connected:       connected,
			LastReason:      lastReason,
			SubscriberCount: subscriberCount,
		},
		DumpConfig: sectionView(config, now),
		Program:    sectionView(program, now),
		Presets:    sectionView(presets, now),
	}
}

func (m *Manager) ProgramRaw() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.program.Raw == "" {
		if m.program.Error != "" {
			return "", errors.New(m.program.Error)
		}
		return "", errors.New("program state is not available yet")
	}
	return m.program.Raw, nil
}

func (m *Manager) ConfigRaw() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.config.Raw == "" {
		if m.config.Error != "" {
			return "", errors.New(m.config.Error)
		}
		return "", errors.New("config state is not available yet")
	}
	return m.config.Raw, nil
}

func (m *Manager) PresetsRaw() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.presets.Raw == "" {
		if m.presets.Error != "" {
			return "", errors.New(m.presets.Error)
		}
		return "", errors.New("preset state is not available yet")
	}
	return m.presets.Raw, nil
}

func (m *Manager) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 16)
	m.subsMu.Lock()
	m.subs[ch] = struct{}{}
	m.subsMu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			m.subsMu.Lock()
			if _, ok := m.subs[ch]; ok {
				delete(m.subs, ch)
				close(ch)
			}
			m.subsMu.Unlock()
		})
	}
	return ch, cancel
}

func (m *Manager) broadcast(event Event) {
	m.subsMu.Lock()
	defer m.subsMu.Unlock()
	for ch := range m.subs {
		select {
		case ch <- event:
		default:
			// Slow clients get the next authoritative event instead of blocking control.
		}
	}
}

func (m *Manager) closeSubscribers() {
	m.subsMu.Lock()
	defer m.subsMu.Unlock()
	for ch := range m.subs {
		close(ch)
		delete(m.subs, ch)
	}
}

func (m *Manager) resolveProgram(raw string) map[string]map[string]string {
	m.mu.RLock()
	resolver := m.resolver
	m.mu.RUnlock()

	var resolved map[string]map[string]string
	if resolver != nil {
		resolved = cloneResolved(resolver(raw))
	}
	if resolved == nil {
		resolved = make(map[string]map[string]string)
	}

	preset := presetFromProgram(raw)
	m.mu.Lock()
	if preset != m.overridePreset {
		m.overrides = make(map[string]map[string]string)
		m.overridePreset = preset
	}
	overrides := cloneResolved(m.overrides)
	m.mu.Unlock()

	for plugin, params := range overrides {
		if resolved[plugin] == nil {
			resolved[plugin] = make(map[string]string)
		}
		for param, value := range params {
			resolved[plugin][param] = value
		}
	}
	if len(resolved) == 0 {
		return nil
	}
	return resolved
}

func presetFromProgram(raw string) string {
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "SetPreset ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "SetPreset "))
		}
	}
	return ""
}

func cloneResolved(in map[string]map[string]string) map[string]map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]map[string]string, len(in))
	for plugin, params := range in {
		out[plugin] = make(map[string]string, len(params))
		for param, value := range params {
			out[plugin][param] = value
		}
	}
	return out
}

func resolvedEqual(a, b map[string]map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for plugin, paramsA := range a {
		paramsB, ok := b[plugin]
		if !ok || len(paramsA) != len(paramsB) {
			return false
		}
		for param, valueA := range paramsA {
			if paramsB[param] != valueA {
				return false
			}
		}
	}
	return true
}

func sectionView(section Section, now time.Time) SectionView {
	view := SectionView{
		Raw:      section.Raw,
		Resolved: cloneResolved(section.Resolved),
		Duration: section.Duration,
		Error:    section.Error,
		Cached:   true,
		Stale:    section.Error != "" && section.Raw != "",
	}
	if !section.UpdatedAt.IsZero() {
		view.UpdatedAt = section.UpdatedAt.Format(time.RFC3339Nano)
		view.AgeMS = now.Sub(section.UpdatedAt).Milliseconds()
	}
	if !section.AttemptedAt.IsZero() {
		view.AttemptedAt = section.AttemptedAt.Format(time.RFC3339Nano)
		view.AttemptAgeMS = now.Sub(section.AttemptedAt).Milliseconds()
	}
	return view
}
