package k8s

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Event is a simplified Kubernetes event for AI and dashboard consumption.
type Event struct {
	Reason         string    `json:"reason"`
	Message        string    `json:"message"`
	Type           string    `json:"type"` // Normal, Warning
	Count          int32     `json:"count"`
	FirstSeen      time.Time `json:"first_seen"`
	LastSeen       time.Time `json:"last_seen"`
	InvolvedObject ObjectRef `json:"involved_object"`
	Source         string    `json:"source"`
}

// ObjectRef identifies the Kubernetes resource an event relates to.
type ObjectRef struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// EventFilter controls which events are returned by ListEvents.
type EventFilter struct {
	Namespace      string
	InvolvedName   string
	InvolvedKind   string
	Reason         string
	EventType      string // "Normal" or "Warning"
	Since          time.Duration
	WarningsOnly   bool
}

// ListEvents returns Kubernetes events matching the filter criteria.
// An empty namespace returns events from all namespaces.
func (c *Client) ListEvents(ctx context.Context, filter EventFilter) ([]Event, error) {
	opts := metav1.ListOptions{}

	// Build a field selector to push filtering to the API server.
	var selectors []string
	if filter.InvolvedName != "" {
		selectors = append(selectors, fmt.Sprintf("involvedObject.name=%s", filter.InvolvedName))
	}
	if filter.InvolvedKind != "" {
		selectors = append(selectors, fmt.Sprintf("involvedObject.kind=%s", filter.InvolvedKind))
	}
	if filter.Reason != "" {
		selectors = append(selectors, fmt.Sprintf("reason=%s", filter.Reason))
	}
	if filter.EventType != "" {
		selectors = append(selectors, fmt.Sprintf("type=%s", filter.EventType))
	}
	if len(selectors) > 0 {
		sel := ""
		for i, s := range selectors {
			if i > 0 {
				sel += ","
			}
			sel += s
		}
		opts.FieldSelector = sel
	}

	list, err := c.Core.CoreV1().Events(filter.Namespace).List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("listing events in namespace %q: %w", filter.Namespace, err)
	}

	cutoff := time.Time{}
	if filter.Since > 0 {
		cutoff = time.Now().Add(-filter.Since)
	}

	events := make([]Event, 0, len(list.Items))
	for _, e := range list.Items {
		if filter.WarningsOnly && e.Type != "Warning" {
			continue
		}
		lastSeen := eventLastSeen(e)
		if !cutoff.IsZero() && lastSeen.Before(cutoff) {
			continue
		}
		events = append(events, toEvent(e))
	}
	return events, nil
}

// GetEventsForResource returns events related to a specific resource.
func (c *Client) GetEventsForResource(ctx context.Context, namespace, name, kind string) ([]Event, error) {
	return c.ListEvents(ctx, EventFilter{
		Namespace:    namespace,
		InvolvedName: name,
		InvolvedKind: kind,
	})
}

// GetWarningEvents returns only Warning-type events, optionally since a given duration.
func (c *Client) GetWarningEvents(ctx context.Context, namespace string, since time.Duration) ([]Event, error) {
	return c.ListEvents(ctx, EventFilter{
		Namespace:    namespace,
		WarningsOnly: true,
		Since:        since,
	})
}

func toEvent(e corev1.Event) Event {
	return Event{
		Reason:    e.Reason,
		Message:   e.Message,
		Type:      e.Type,
		Count:     e.Count,
		FirstSeen: e.FirstTimestamp.Time,
		LastSeen:  eventLastSeen(e),
		InvolvedObject: ObjectRef{
			Kind:      e.InvolvedObject.Kind,
			Name:      e.InvolvedObject.Name,
			Namespace: e.InvolvedObject.Namespace,
		},
		Source: e.Source.Component,
	}
}

func eventLastSeen(e corev1.Event) time.Time {
	if !e.LastTimestamp.IsZero() {
		return e.LastTimestamp.Time
	}
	if e.EventTime.Time.IsZero() {
		return e.FirstTimestamp.Time
	}
	return e.EventTime.Time
}
