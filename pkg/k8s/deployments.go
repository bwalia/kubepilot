package k8s

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeploymentSummary is a concise view of a Deployment for the dashboard and AI.
type DeploymentSummary struct {
	Name              string
	Namespace         string
	Replicas          int32
	ReadyReplicas     int32
	AvailableReplicas int32
	Image             string
}

// ListDeployments returns all deployments in a namespace (or all namespaces if empty).
func (c *Client) ListDeployments(ctx context.Context, namespace string) ([]DeploymentSummary, error) {
	list, err := c.Core.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing deployments in namespace %q: %w", namespace, err)
	}

	summaries := make([]DeploymentSummary, 0, len(list.Items))
	for _, d := range list.Items {
		summaries = append(summaries, toDeploymentSummary(d))
	}
	return summaries, nil
}

// ScaleDeployment sets the replica count for a deployment.
// Callers MUST have validated a CR code before invoking this on production namespaces.
func (c *Client) ScaleDeployment(ctx context.Context, namespace, name string, replicas int32) error {
	scale, err := c.Core.AppsV1().Deployments(namespace).GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting scale for deployment %s/%s: %w", namespace, name, err)
	}

	scale.Spec.Replicas = replicas
	_, err = c.Core.AppsV1().Deployments(namespace).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("scaling deployment %s/%s to %d replicas: %w", namespace, name, replicas, err)
	}
	return nil
}

// RestartDeployment triggers a rolling restart by patching the pod template annotation.
// Callers MUST have validated a CR code before invoking this on production namespaces.
func (c *Client) RestartDeployment(ctx context.Context, namespace, name string) error {
	deployment, err := c.Core.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting deployment %s/%s: %w", namespace, name, err)
	}

	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	// kubectl rollout restart works by bumping this annotation, triggering new pods.
	deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = metav1.Now().UTC().Format("2006-01-02T15:04:05Z")

	_, err = c.Core.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("patching restart annotation on deployment %s/%s: %w", namespace, name, err)
	}
	return nil
}

func toDeploymentSummary(d appsv1.Deployment) DeploymentSummary {
	image := ""
	if len(d.Spec.Template.Spec.Containers) > 0 {
		image = d.Spec.Template.Spec.Containers[0].Image
	}
	return DeploymentSummary{
		Name:              d.Name,
		Namespace:         d.Namespace,
		Replicas:          d.Status.Replicas,
		ReadyReplicas:     d.Status.ReadyReplicas,
		AvailableReplicas: d.Status.AvailableReplicas,
		Image:             image,
	}
}
