package metrics

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

type counterKey struct {
	name   string
	labels string
}

type Registry struct {
	mu       sync.RWMutex
	counters map[counterKey]*uint64
	help     map[string]string
}

var Default = NewRegistry()

func NewRegistry() *Registry {
	return &Registry{
		counters: make(map[counterKey]*uint64),
		help:     make(map[string]string),
	}
}

func (r *Registry) Register(name, help string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.help[name] = help
}

func (r *Registry) Inc(name string, labels map[string]string) {
	r.Add(name, labels, 1)
}

func (r *Registry) Add(name string, labels map[string]string, delta uint64) {
	key := counterKey{name: name, labels: encodeLabels(labels)}
	r.mu.RLock()
	ptr, ok := r.counters[key]
	r.mu.RUnlock()
	if !ok {
		r.mu.Lock()
		ptr, ok = r.counters[key]
		if !ok {
			var v uint64
			ptr = &v
			r.counters[key] = ptr
		}
		r.mu.Unlock()
	}
	atomic.AddUint64(ptr, delta)
}

func (r *Registry) Snapshot() map[string]uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]uint64, len(r.counters))
	for k, v := range r.counters {
		name := k.name
		if k.labels != "" {
			name = name + "{" + k.labels + "}"
		}
		out[name] = atomic.LoadUint64(v)
	}
	return out
}

func (r *Registry) WritePrometheus(w io.Writer) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	byName := make(map[string][]counterKey)
	for k := range r.counters {
		byName[k.name] = append(byName[k.name], k)
	}

	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		if help, ok := r.help[name]; ok {
			fmt.Fprintf(w, "# HELP %s %s\n", name, help)
		}
		fmt.Fprintf(w, "# TYPE %s counter\n", name)

		keys := byName[name]
		sort.Slice(keys, func(i, j int) bool { return keys[i].labels < keys[j].labels })
		for _, k := range keys {
			value := atomic.LoadUint64(r.counters[k])
			if k.labels == "" {
				fmt.Fprintf(w, "%s %d\n", k.name, value)
			} else {
				fmt.Fprintf(w, "%s{%s} %d\n", k.name, k.labels, value)
			}
		}
	}
}

func (r *Registry) WriteJSON(w io.Writer) error {
	snap := r.Snapshot()
	return json.NewEncoder(w).Encode(snap)
}

func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		Default.WritePrometheus(w)
	})
}

func JSONHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = Default.WriteJSON(w)
	})
}

func encodeLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(labels))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%q", k, labels[k]))
	}
	return strings.Join(parts, ",")
}

const (
	TransferAttempts          = "transfer_attempts_total"
	TransferConflicts         = "transfer_conflicts_total"
	TransferSuccess           = "transfer_success_total"
	TransferFailures          = "transfer_failures_total"
	TransferRetriesExhausted  = "transfer_retries_exhausted_total"
	TransferLockAcquireFailed = "transfer_lock_acquire_failed_total"
)

func init() {
	Default.Register(TransferAttempts, "Total transfer attempts by locking mode")
	Default.Register(TransferConflicts, "Total retryable concurrency conflicts by locking mode")
	Default.Register(TransferSuccess, "Total successful transfers by locking mode")
	Default.Register(TransferFailures, "Total failed transfers by reason")
	Default.Register(TransferRetriesExhausted, "Transfers marked FAILED after exhausting retry budget")
	Default.Register(TransferLockAcquireFailed, "Pessimistic lock acquisition failures")
}
