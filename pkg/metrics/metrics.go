// Package metrics exposes a handful of counters and gauges in Prometheus text
// exposition format.
//
// Hand-rolled rather than pulling in the Prometheus client library: the service
// needs a few counters and one gauge, the format is three lines of encoding, and
// the client library brings a large dependency tree for features nothing here
// uses. Swap it in if histograms or exemplars ever become necessary.
package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// Counter is a monotonically increasing value.
type Counter struct {
	name string
	help string
	v    atomic.Int64
}

func (c *Counter) Inc()         { c.v.Add(1) }
func (c *Counter) Add(n int64)  { c.v.Add(n) }
func (c *Counter) Value() int64 { return c.v.Load() }

// GaugeFunc reports a value sampled at scrape time — for things the process does
// not own, like the depth of a Redis list.
type GaugeFunc struct {
	name   string
	help   string
	labels map[string]string
	fn     func() (float64, error)
}

// Registry holds the metrics one process exposes.
type Registry struct {
	mu       sync.RWMutex
	counters []*Counter
	gauges   []*GaugeFunc
}

func NewRegistry() *Registry { return &Registry{} }

func (r *Registry) NewCounter(name, help string) *Counter {
	c := &Counter{name: name, help: help}
	r.mu.Lock()
	r.counters = append(r.counters, c)
	r.mu.Unlock()
	return c
}

// RegisterGauge adds a value sampled on every scrape. A sampling error is
// reported by omitting the metric rather than by serving a stale or zero value,
// which would look like a healthy empty queue.
func (r *Registry) RegisterGauge(name, help string, labels map[string]string, fn func() (float64, error)) {
	r.mu.Lock()
	r.gauges = append(r.gauges, &GaugeFunc{name: name, help: help, labels: labels, fn: fn})
	r.mu.Unlock()
}

// Handler serves the registry in Prometheus text exposition format.
func (r *Registry) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		r.mu.RLock()
		counters := append([]*Counter(nil), r.counters...)
		gauges := append([]*GaugeFunc(nil), r.gauges...)
		r.mu.RUnlock()

		var b strings.Builder
		for _, c := range counters {
			fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s counter\n%s %d\n",
				c.name, c.help, c.name, c.name, c.Value())
		}
		for _, g := range gauges {
			value, err := g.fn()
			if err != nil {
				continue // an unsampled gauge is absent, never zero
			}
			fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s gauge\n%s%s %g\n",
				g.name, g.help, g.name, g.name, formatLabels(g.labels), value)
		}
		_, _ = w.Write([]byte(b.String()))
	}
}

// formatLabels renders labels in a stable order so scrape output does not churn.
func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%q", k, labels[k]))
	}
	return "{" + strings.Join(parts, ",") + "}"
}
