package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPodOwnerCollapsesReplicaSetToDeployment(t *testing.T) {
	ctrl := true
	p := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Labels:          map[string]string{"pod-template-hash": "7d9f8c"},
		OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "api-7d9f8c", Controller: &ctrl}},
	}}
	if kind, name := podOwner(p); kind != "Deployment" || name != "api" {
		t.Fatalf("got %s/%s, want Deployment/api", kind, name)
	}

	p.OwnerReferences[0] = metav1.OwnerReference{Kind: "StatefulSet", Name: "db", Controller: &ctrl}
	if kind, name := podOwner(p); kind != "StatefulSet" || name != "db" {
		t.Fatalf("got %s/%s, want StatefulSet/db", kind, name)
	}
}

func TestListPodUsageFallsBackToPVCCapacity(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "db-0", Namespace: "data"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "db",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("250m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
					Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
				},
			}},
			Volumes: []corev1.Volume{{
				Name:         "data",
				VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data-db-0"}},
			}},
		},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "data-db-0", Namespace: "data"},
		Status: corev1.PersistentVolumeClaimStatus{
			Capacity: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
		},
	}
	c := &Client{Core: fake.NewSimpleClientset(pod, pvc)}

	report, err := c.ListPodUsage(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if report.MetricsAvailable || report.StorageStatsAvailable {
		t.Fatalf("no metrics sources configured, got %+v", report)
	}
	if len(report.Pods) != 1 {
		t.Fatalf("want 1 pod, got %d", len(report.Pods))
	}
	got := report.Pods[0]
	if got.CPURequestMilli != 250 || got.CPULimitMilli != 1000 || got.MemRequestBytes != 512<<20 {
		t.Fatalf("requests/limits wrong: %+v", got)
	}
	if !got.StorageEstimated || got.StorageUsedBytes != 10<<30 || got.PVCCount != 1 {
		t.Fatalf("storage fallback wrong: %+v", got)
	}
}

func TestSharedFilesystemDetectsLocalPathVolumes(t *testing.T) {
	const gi = int64(1) << 30
	localPath := pvcVolumeStats{claim: "data", used: 1700 * gi, capacity: 1800 * gi, hasUsed: true, hasCapacity: true}
	if !sharedFilesystem(localPath, 10*gi) {
		t.Fatal("a 1.8TiB filesystem behind a 10GiB claim should be treated as the host disk")
	}
	block := pvcVolumeStats{claim: "data", used: 3 * gi, capacity: 10 * gi, hasUsed: true, hasCapacity: true}
	if sharedFilesystem(block, 10*gi) {
		t.Fatal("a dedicated 10GiB volume should be trusted")
	}
}
