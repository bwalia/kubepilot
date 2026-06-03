package k8s

import (
	"context"
	"fmt"

	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigMapSummary is a concise view of a ConfigMap. Only key names/count are
// surfaced — values are not part of this summary to keep payloads lean.
type ConfigMapSummary struct {
	Name      string `json:"Name"`
	Namespace string `json:"Namespace"`
	KeyCount  int    `json:"KeyCount"`
}

// SecretSummary is a deliberately data-free view of a Secret. The Data and
// StringData fields are NEVER included so secret values can never leak through
// the dashboard API. Only the key count is exposed.
type SecretSummary struct {
	Name      string `json:"Name"`
	Namespace string `json:"Namespace"`
	Type      string `json:"Type"`
	KeyCount  int    `json:"KeyCount"`
}

// PVCSummary is a concise view of a PersistentVolumeClaim for the dashboard browser.
type PVCSummary struct {
	Name         string   `json:"Name"`
	Namespace    string   `json:"Namespace"`
	Status       string   `json:"Status"`
	StorageClass string   `json:"StorageClass"`
	Capacity     string   `json:"Capacity"`
	VolumeName   string   `json:"VolumeName"`
	AccessModes  []string `json:"AccessModes"`
}

// StorageClassInfo is a concise view of a StorageClass for the dashboard browser.
type StorageClassInfo struct {
	Name                 string `json:"Name"`
	Provisioner          string `json:"Provisioner"`
	ReclaimPolicy        string `json:"ReclaimPolicy"`
	VolumeBindingMode    string `json:"VolumeBindingMode"`
	AllowVolumeExpansion bool   `json:"AllowVolumeExpansion"`
}

// ListConfigMaps returns ConfigMap metadata (no values) in a namespace (or all if empty).
func (c *Client) ListConfigMaps(ctx context.Context, namespace string) ([]ConfigMapSummary, error) {
	list, err := c.Core.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing configmaps in namespace %q: %w", namespace, err)
	}

	summaries := make([]ConfigMapSummary, 0, len(list.Items))
	for _, cm := range list.Items {
		summaries = append(summaries, ConfigMapSummary{
			Name:      cm.Name,
			Namespace: cm.Namespace,
			KeyCount:  len(cm.Data) + len(cm.BinaryData),
		})
	}
	return summaries, nil
}

// ListSecrets returns Secret metadata in a namespace (or all if empty).
//
// SECURITY: this method intentionally never reads or returns the Secret Data or
// StringData fields. Only the name, namespace, type, and number of keys are
// surfaced. Do not add value-bearing fields to SecretSummary.
func (c *Client) ListSecrets(ctx context.Context, namespace string) ([]SecretSummary, error) {
	list, err := c.Core.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing secrets in namespace %q: %w", namespace, err)
	}

	summaries := make([]SecretSummary, 0, len(list.Items))
	for _, s := range list.Items {
		summaries = append(summaries, SecretSummary{
			Name:      s.Name,
			Namespace: s.Namespace,
			Type:      string(s.Type),
			KeyCount:  len(s.Data),
		})
	}
	return summaries, nil
}

// ListPVCs returns PersistentVolumeClaims in a namespace (or all if empty).
func (c *Client) ListPVCs(ctx context.Context, namespace string) ([]PVCSummary, error) {
	list, err := c.Core.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pvcs in namespace %q: %w", namespace, err)
	}

	summaries := make([]PVCSummary, 0, len(list.Items))
	for _, pvc := range list.Items {
		summary := PVCSummary{
			Name:       pvc.Name,
			Namespace:  pvc.Namespace,
			Status:     string(pvc.Status.Phase),
			VolumeName: pvc.Spec.VolumeName,
		}
		if pvc.Spec.StorageClassName != nil {
			summary.StorageClass = *pvc.Spec.StorageClassName
		}
		if qty, ok := pvc.Status.Capacity["storage"]; ok {
			summary.Capacity = qty.String()
		} else if qty, ok := pvc.Spec.Resources.Requests["storage"]; ok {
			summary.Capacity = qty.String()
		}
		for _, mode := range pvc.Spec.AccessModes {
			summary.AccessModes = append(summary.AccessModes, string(mode))
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

// ListStorageClasses returns all StorageClasses in the cluster.
func (c *Client) ListStorageClasses(ctx context.Context) ([]StorageClassInfo, error) {
	list, err := c.Core.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing storage classes: %w", err)
	}

	infos := make([]StorageClassInfo, 0, len(list.Items))
	for _, sc := range list.Items {
		infos = append(infos, toStorageClassInfo(sc))
	}
	return infos, nil
}

func toStorageClassInfo(sc storagev1.StorageClass) StorageClassInfo {
	info := StorageClassInfo{
		Name:        sc.Name,
		Provisioner: sc.Provisioner,
	}
	if sc.ReclaimPolicy != nil {
		info.ReclaimPolicy = string(*sc.ReclaimPolicy)
	}
	if sc.VolumeBindingMode != nil {
		info.VolumeBindingMode = string(*sc.VolumeBindingMode)
	}
	if sc.AllowVolumeExpansion != nil {
		info.AllowVolumeExpansion = *sc.AllowVolumeExpansion
	}
	return info
}
