package timeline

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/BigKAA/dephealth-ui/internal/topology"
)

// Event represents a state transition detected on the timeline.
type Event struct {
	Timestamp time.Time `json:"timestamp"`
	Service   string    `json:"service"`
	Namespace string    `json:"namespace,omitempty"`
	FromState string    `json:"fromState"`
	ToState   string    `json:"toState"`
	Kind      string    `json:"kind"` // "degradation", "recovery", "change"
}

// EventsRequest holds parameters for querying timeline events.
type EventsRequest struct {
	Start     time.Time
	End       time.Time
	Namespace string
}

// statusSeverity returns a numeric severity for status ordering.
// Higher values indicate worse states. Used for kind classification.
func statusSeverity(status string) int {
	switch status {
	case "ok":
		return 0
	case "timeout":
		return 1
	case "unhealthy":
		return 2
	case "connection_error", "dns_error", "auth_error", "tls_error":
		return 3
	case "error":
		return 4
	default:
		return -1 // unknown
	}
}

// classifyTransition determines the kind of state transition.
func classifyTransition(from, to string) string {
	fromSev := statusSeverity(from)
	toSev := statusSeverity(to)

	switch {
	case toSev > fromSev:
		return "degradation"
	case toSev < fromSev:
		return "recovery"
	default:
		return "change"
	}
}

// AutoStep returns an appropriate query step for the given time range,
// balancing resolution and VM query performance.
func AutoStep(d time.Duration) time.Duration {
	switch {
	case d <= time.Hour:
		return 15 * time.Second
	case d <= 6*time.Hour:
		return time.Minute
	case d <= 24*time.Hour:
		return 5 * time.Minute
	case d <= 7*24*time.Hour:
		return 15 * time.Minute
	case d <= 30*24*time.Hour:
		return time.Hour
	case d <= 90*24*time.Hour:
		return 3 * time.Hour
	default:
		return 6 * time.Hour
	}
}

// QueryStatusTransitions queries the dependency status metric over a time range
// and detects state transitions. Returns a sorted list of events.
func QueryStatusTransitions(ctx context.Context, prom topology.PrometheusClient, req EventsRequest) ([]Event, error) {
	rangeDuration := req.End.Sub(req.Start)
	if rangeDuration <= 0 {
		return nil, fmt.Errorf("invalid range: start must be before end")
	}

	step := AutoStep(rangeDuration)
	results, err := prom.QueryStatusRange(ctx, req.Start, req.End, step, req.Namespace)
	if err != nil {
		return nil, fmt.Errorf("querying status range: %w", err)
	}

	// Group range results by EdgeKey to track status changes per edge.
	// Each edge can have multiple series (one per status value), but only
	// the one with value == 1 is "active" at any given time.
	type edgeKey struct {
		Name string
		Host string
		Port string
	}

	// Build a map: edgeKey → timestamp → active status.
	edgeStatusAtTime := make(map[edgeKey]map[int64]string)

	for _, r := range results {
		ek := edgeKey{
			Name: r.Key.Name,
			Host: r.Key.Host,
			Port: r.Key.Port,
		}

		if _, ok := edgeStatusAtTime[ek]; !ok {
			edgeStatusAtTime[ek] = make(map[int64]string)
		}

		for _, tv := range r.Values {
			if tv.Value == 1 {
				edgeStatusAtTime[ek][tv.Timestamp.Unix()] = r.Status
			}
		}
	}

	// Collect all unique timestamps across all edges.
	allTimestamps := make(map[int64]bool)
	for _, statusMap := range edgeStatusAtTime {
		for ts := range statusMap {
			allTimestamps[ts] = true
		}
	}

	sortedTS := make([]int64, 0, len(allTimestamps))
	for ts := range allTimestamps {
		sortedTS = append(sortedTS, ts)
	}
	sort.Slice(sortedTS, func(i, j int) bool { return sortedTS[i] < sortedTS[j] })

	// Detect transitions for each edge.
	var events []Event
	for ek, statusMap := range edgeStatusAtTime {
		var prevStatus string
		for _, ts := range sortedTS {
			status, exists := statusMap[ts]
			if !exists {
				continue
			}
			if prevStatus != "" && status != prevStatus {
				events = append(events, Event{
					Timestamp: time.Unix(ts, 0).UTC(),
					Service:   ek.Name,
					FromState: prevStatus,
					ToState:   status,
					Kind:      classifyTransition(prevStatus, status),
				})
			}
			prevStatus = status
		}
	}

	// Sort events by timestamp.
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	return events, nil
}
