package k8s

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StatefulSetSummary is a concise view of a StatefulSet for the dashboard browser.
type StatefulSetSummary struct {
	Name          string `json:"Name"`
	Namespace     string `json:"Namespace"`
	Replicas      int32  `json:"Replicas"`
	ReadyReplicas int32  `json:"ReadyReplicas"`
	Image         string `json:"Image"`
	ServiceName   string `json:"ServiceName"`
}

// DaemonSetSummary is a concise view of a DaemonSet for the dashboard browser.
type DaemonSetSummary struct {
	Name                   string `json:"Name"`
	Namespace              string `json:"Namespace"`
	DesiredNumberScheduled int32  `json:"DesiredNumberScheduled"`
	NumberReady            int32  `json:"NumberReady"`
	Image                  string `json:"Image"`
}

// K8sJobSummary is a concise view of a batch/v1 Job. It is named K8sJobSummary to
// avoid collision with KubePilot's internal job scheduler types in pkg/jobs.
type K8sJobSummary struct {
	Name           string     `json:"Name"`
	Namespace      string     `json:"Namespace"`
	Status         string     `json:"Status"` // Complete | Failed | Active
	Completions    int32      `json:"Completions"`
	Succeeded      int32      `json:"Succeeded"`
	Failed         int32      `json:"Failed"`
	StartTime      *time.Time `json:"StartTime,omitempty"`
	CompletionTime *time.Time `json:"CompletionTime,omitempty"`
}

// CronJobSummary is a concise view of a batch/v1 CronJob for the dashboard browser.
type CronJobSummary struct {
	Name             string     `json:"Name"`
	Namespace        string     `json:"Namespace"`
	Schedule         string     `json:"Schedule"`
	Suspend          bool       `json:"Suspend"`
	Active           int        `json:"Active"`
	LastScheduleTime *time.Time `json:"LastScheduleTime,omitempty"`
}

// ListStatefulSets returns all StatefulSets in a namespace (or all namespaces if empty).
func (c *Client) ListStatefulSets(ctx context.Context, namespace string) ([]StatefulSetSummary, error) {
	list, err := c.Core.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing statefulsets in namespace %q: %w", namespace, err)
	}

	summaries := make([]StatefulSetSummary, 0, len(list.Items))
	for _, s := range list.Items {
		summaries = append(summaries, toStatefulSetSummary(s))
	}
	return summaries, nil
}

// ListDaemonSets returns all DaemonSets in a namespace (or all namespaces if empty).
func (c *Client) ListDaemonSets(ctx context.Context, namespace string) ([]DaemonSetSummary, error) {
	list, err := c.Core.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing daemonsets in namespace %q: %w", namespace, err)
	}

	summaries := make([]DaemonSetSummary, 0, len(list.Items))
	for _, d := range list.Items {
		summaries = append(summaries, toDaemonSetSummary(d))
	}
	return summaries, nil
}

// ListJobs returns all batch/v1 Jobs in a namespace (or all namespaces if empty).
func (c *Client) ListJobs(ctx context.Context, namespace string) ([]K8sJobSummary, error) {
	list, err := c.Core.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing jobs in namespace %q: %w", namespace, err)
	}

	summaries := make([]K8sJobSummary, 0, len(list.Items))
	for _, j := range list.Items {
		summaries = append(summaries, toJobSummary(j))
	}
	return summaries, nil
}

// ListCronJobs returns all batch/v1 CronJobs in a namespace (or all namespaces if empty).
func (c *Client) ListCronJobs(ctx context.Context, namespace string) ([]CronJobSummary, error) {
	list, err := c.Core.BatchV1().CronJobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing cronjobs in namespace %q: %w", namespace, err)
	}

	summaries := make([]CronJobSummary, 0, len(list.Items))
	for _, cj := range list.Items {
		summaries = append(summaries, toCronJobSummary(cj))
	}
	return summaries, nil
}

func toStatefulSetSummary(s appsv1.StatefulSet) StatefulSetSummary {
	image := ""
	if len(s.Spec.Template.Spec.Containers) > 0 {
		image = s.Spec.Template.Spec.Containers[0].Image
	}
	replicas := int32(0)
	if s.Spec.Replicas != nil {
		replicas = *s.Spec.Replicas
	}
	return StatefulSetSummary{
		Name:          s.Name,
		Namespace:     s.Namespace,
		Replicas:      replicas,
		ReadyReplicas: s.Status.ReadyReplicas,
		Image:         image,
		ServiceName:   s.Spec.ServiceName,
	}
}

func toDaemonSetSummary(d appsv1.DaemonSet) DaemonSetSummary {
	image := ""
	if len(d.Spec.Template.Spec.Containers) > 0 {
		image = d.Spec.Template.Spec.Containers[0].Image
	}
	return DaemonSetSummary{
		Name:                   d.Name,
		Namespace:              d.Namespace,
		DesiredNumberScheduled: d.Status.DesiredNumberScheduled,
		NumberReady:            d.Status.NumberReady,
		Image:                  image,
	}
}

func toJobSummary(j batchv1.Job) K8sJobSummary {
	summary := K8sJobSummary{
		Name:      j.Name,
		Namespace: j.Namespace,
		Succeeded: j.Status.Succeeded,
		Failed:    j.Status.Failed,
		Status:    jobStatus(j),
	}
	if j.Spec.Completions != nil {
		summary.Completions = *j.Spec.Completions
	}
	if j.Status.StartTime != nil {
		t := j.Status.StartTime.Time
		summary.StartTime = &t
	}
	if j.Status.CompletionTime != nil {
		t := j.Status.CompletionTime.Time
		summary.CompletionTime = &t
	}
	return summary
}

func jobStatus(j batchv1.Job) string {
	for _, cond := range j.Status.Conditions {
		if cond.Status != "True" {
			continue
		}
		switch cond.Type {
		case batchv1.JobComplete:
			return "Complete"
		case batchv1.JobFailed:
			return "Failed"
		}
	}
	if j.Status.Active > 0 {
		return "Active"
	}
	return "Pending"
}

func toCronJobSummary(cj batchv1.CronJob) CronJobSummary {
	summary := CronJobSummary{
		Name:      cj.Name,
		Namespace: cj.Namespace,
		Schedule:  cj.Spec.Schedule,
		Active:    len(cj.Status.Active),
	}
	if cj.Spec.Suspend != nil {
		summary.Suspend = *cj.Spec.Suspend
	}
	if cj.Status.LastScheduleTime != nil {
		t := cj.Status.LastScheduleTime.Time
		summary.LastScheduleTime = &t
	}
	return summary
}
