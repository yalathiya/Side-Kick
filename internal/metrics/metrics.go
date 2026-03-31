package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	dto "github.com/prometheus/client_model/go"
)

// Metrics holds all Prometheus metrics for Sidekick.
type Metrics struct {
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	RateLimitHits   prometheus.Counter
}

// RouteStats holds per-route metric data for the dashboard.
type RouteStats struct {
	Method   string  `json:"method"`
	Route    string  `json:"route"`
	Status   string  `json:"status"`
	Count    float64 `json:"count"`
	AvgMs    float64 `json:"avg_ms"`
	TotalMs  float64 `json:"total_ms"`
	Buckets  map[string]uint64 `json:"buckets,omitempty"`
}

// Snapshot holds a point-in-time snapshot of all sidekick metrics.
type Snapshot struct {
	TotalRequests  float64      `json:"total_requests"`
	RateLimitHits  float64      `json:"ratelimit_hits"`
	Routes         []RouteStats `json:"routes"`
	UptimeSeconds  float64      `json:"uptime_seconds"`
}

// Snapshot collects current metric values into a JSON-friendly struct.
func (m *Metrics) Snapshot() Snapshot {
	snap := Snapshot{}

	// Gather rate limit hits
	var rlMetric dto.Metric
	if err := m.RateLimitHits.Write(&rlMetric); err == nil && rlMetric.Counter != nil {
		snap.RateLimitHits = rlMetric.Counter.GetValue()
	}

	// Gather per-route request counts and durations
	reqCh := make(chan prometheus.Metric, 100)
	go func() { m.RequestsTotal.Collect(reqCh); close(reqCh) }()

	countMap := map[string]float64{}    // "method|route|status" -> count
	for metric := range reqCh {
		var dm dto.Metric
		if err := metric.Write(&dm); err != nil {
			continue
		}
		labels := labelMap(dm.GetLabel())
		key := labels["method"] + "|" + labels["route"] + "|" + labels["status"]
		countMap[key] = dm.Counter.GetValue()
		snap.TotalRequests += dm.Counter.GetValue()
	}

	durCh := make(chan prometheus.Metric, 100)
	go func() { m.RequestDuration.Collect(durCh); close(durCh) }()

	durMap := map[string]*dto.Metric{} // key -> histogram metric
	for metric := range durCh {
		var dm dto.Metric
		if err := metric.Write(&dm); err != nil {
			continue
		}
		labels := labelMap(dm.GetLabel())
		key := labels["method"] + "|" + labels["route"] + "|" + labels["status"]
		copied := dm
		durMap[key] = &copied
	}

	for key, count := range countMap {
		parts := splitKey(key)
		rs := RouteStats{
			Method: parts[0],
			Route:  parts[1],
			Status: parts[2],
			Count:  count,
		}
		if dm, ok := durMap[key]; ok && dm.Histogram != nil {
			totalSec := dm.Histogram.GetSampleSum()
			rs.TotalMs = totalSec * 1000
			if count > 0 {
				rs.AvgMs = (totalSec / count) * 1000
			}
		}
		snap.Routes = append(snap.Routes, rs)
	}

	return snap
}

func labelMap(labels []*dto.LabelPair) map[string]string {
	m := make(map[string]string, len(labels))
	for _, lp := range labels {
		m[lp.GetName()] = lp.GetValue()
	}
	return m
}

func splitKey(key string) [3]string {
	var parts [3]string
	idx := 0
	start := 0
	for i, c := range key {
		if c == '|' {
			parts[idx] = key[start:i]
			idx++
			start = i + 1
		}
	}
	parts[idx] = key[start:]
	return parts
}

// New registers and returns Sidekick metrics using the default Prometheus registry.
func New() *Metrics {
	return NewWithRegistry(prometheus.DefaultRegisterer)
}

// NewWithRegistry registers metrics with a custom Prometheus registerer.
// Useful for testing with isolated registries.
func NewWithRegistry(reg prometheus.Registerer) *Metrics {
	factory := promauto.With(reg)
	return &Metrics{
		RequestsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sidekick_requests_total",
				Help: "Total number of requests processed by Sidekick",
			},
			[]string{"method", "route", "status"},
		),
		RequestDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "sidekick_request_duration_seconds",
				Help:    "Request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "route", "status"},
		),
		RateLimitHits: factory.NewCounter(
			prometheus.CounterOpts{
				Name: "sidekick_ratelimit_hits_total",
				Help: "Total number of requests rejected by rate limiter",
			},
		),
	}
}
