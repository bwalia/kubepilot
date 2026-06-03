package k8s

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NamespaceSummary is a concise view of a Namespace for the dashboard browser.
type NamespaceSummary struct {
	Name      string    `json:"Name"`
	Status    string    `json:"Status"`
	CreatedAt time.Time `json:"CreatedAt"`
}

// ListNamespaces returns all namespaces in the cluster. It drives the
// namespace selector that filters every other resource list in the dashboard.
func (c *Client) ListNamespaces(ctx context.Context) ([]NamespaceSummary, error) {
	list, err := c.Core.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing namespaces: %w", err)
	}

	summaries := make([]NamespaceSummary, 0, len(list.Items))
	for _, ns := range list.Items {
		summaries = append(summaries, toNamespaceSummary(ns))
	}
	return summaries, nil
}

func toNamespaceSummary(ns corev1.Namespace) NamespaceSummary {
	return NamespaceSummary{
		Name:      ns.Name,
		Status:    string(ns.Status.Phase),
		CreatedAt: ns.CreationTimestamp.Time,
	}
}
