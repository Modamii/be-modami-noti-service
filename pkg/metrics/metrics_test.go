package metrics

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func scrape(t *testing.T, r *Registry) string {
	t.Helper()
	rec := httptest.NewRecorder()
	r.Handler()(rec, httptest.NewRequest("GET", "/metrics", nil))
	return rec.Body.String()
}

func TestCounterExposition(t *testing.T) {
	r := NewRegistry()
	c := r.NewCounter("notif_push_sent_total", "Pushes delivered")
	c.Inc()
	c.Add(4)

	body := scrape(t, r)
	for _, want := range []string{
		"# HELP notif_push_sent_total Pushes delivered",
		"# TYPE notif_push_sent_total counter",
		"notif_push_sent_total 5",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("scrape missing %q:\n%s", want, body)
		}
	}
}

func TestGaugeLabelsAreStable(t *testing.T) {
	r := NewRegistry()
	r.RegisterGauge("notif_queue_depth", "Pending messages",
		map[string]string{"queue": "ws", "env": "prod"},
		func() (float64, error) { return 12, nil })

	first := scrape(t, r)
	if !strings.Contains(first, `notif_queue_depth{env="prod",queue="ws"} 12`) {
		t.Errorf("labels not sorted or value wrong:\n%s", first)
	}
	// Map iteration order must not leak into the output, or every scrape looks
	// like a changed series.
	if second := scrape(t, r); first != second {
		t.Errorf("scrape output is not stable:\n%s\n---\n%s", first, second)
	}
}

func TestFailedGaugeIsOmittedNotZeroed(t *testing.T) {
	r := NewRegistry()
	r.RegisterGauge("notif_queue_depth", "Pending messages", nil,
		func() (float64, error) { return 0, errors.New("redis down") })
	r.NewCounter("notif_dispatch_total", "Dispatched").Inc()

	body := scrape(t, r)
	// Reporting 0 for an unreachable queue would read as "healthy and empty" —
	// exactly the state an alert is meant to distinguish from.
	if strings.Contains(body, "notif_queue_depth") {
		t.Errorf("an unsampled gauge must be absent, not zero:\n%s", body)
	}
	if !strings.Contains(body, "notif_dispatch_total 1") {
		t.Errorf("one broken gauge must not suppress the rest:\n%s", body)
	}
}
