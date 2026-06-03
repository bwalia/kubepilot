package k8s

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestListStatefulSets(t *testing.T) {
	replicas := int32(3)
	client := &Client{Core: fake.NewSimpleClientset(&appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: "db-headless",
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "postgres:16"}}},
			},
		},
		Status: appsv1.StatefulSetStatus{ReadyReplicas: 2},
	})}

	sets, err := client.ListStatefulSets(context.Background(), "default")
	if err != nil {
		t.Fatalf("ListStatefulSets returned error: %v", err)
	}
	if len(sets) != 1 {
		t.Fatalf("expected 1 statefulset, got %d", len(sets))
	}
	got := sets[0]
	if got.Replicas != 3 || got.ReadyReplicas != 2 {
		t.Errorf("replicas = %d/%d, want 2/3", got.ReadyReplicas, got.Replicas)
	}
	if got.Image != "postgres:16" {
		t.Errorf("Image = %q, want postgres:16", got.Image)
	}
	if got.ServiceName != "db-headless" {
		t.Errorf("ServiceName = %q, want db-headless", got.ServiceName)
	}
}

func TestListDaemonSets(t *testing.T) {
	client := &Client{Core: fake.NewSimpleClientset(&appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "node-agent", Namespace: "kube-system"},
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "agent:1.0"}}},
			},
		},
		Status: appsv1.DaemonSetStatus{DesiredNumberScheduled: 3, NumberReady: 3},
	})}

	sets, err := client.ListDaemonSets(context.Background(), "")
	if err != nil {
		t.Fatalf("ListDaemonSets returned error: %v", err)
	}
	if len(sets) != 1 {
		t.Fatalf("expected 1 daemonset, got %d", len(sets))
	}
	if sets[0].DesiredNumberScheduled != 3 || sets[0].NumberReady != 3 {
		t.Errorf("scheduled/ready = %d/%d, want 3/3", sets[0].DesiredNumberScheduled, sets[0].NumberReady)
	}
}

func TestListJobs_StatusDerivation(t *testing.T) {
	completions := int32(1)
	client := &Client{Core: fake.NewSimpleClientset(
		&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{Name: "done", Namespace: "default"},
			Spec:       batchv1.JobSpec{Completions: &completions},
			Status: batchv1.JobStatus{
				Succeeded:  1,
				Conditions: []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}},
			},
		},
		&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{Name: "running", Namespace: "default"},
			Status:     batchv1.JobStatus{Active: 1},
		},
	)}

	jobs, err := client.ListJobs(context.Background(), "default")
	if err != nil {
		t.Fatalf("ListJobs returned error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	status := map[string]string{}
	for _, j := range jobs {
		status[j.Name] = j.Status
	}
	if status["done"] != "Complete" {
		t.Errorf("done status = %q, want Complete", status["done"])
	}
	if status["running"] != "Active" {
		t.Errorf("running status = %q, want Active", status["running"])
	}
}

func TestListCronJobs(t *testing.T) {
	suspend := true
	client := &Client{Core: fake.NewSimpleClientset(&batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "default"},
		Spec:       batchv1.CronJobSpec{Schedule: "0 2 * * *", Suspend: &suspend},
	})}

	cjs, err := client.ListCronJobs(context.Background(), "default")
	if err != nil {
		t.Fatalf("ListCronJobs returned error: %v", err)
	}
	if len(cjs) != 1 {
		t.Fatalf("expected 1 cronjob, got %d", len(cjs))
	}
	if cjs[0].Schedule != "0 2 * * *" {
		t.Errorf("Schedule = %q, want '0 2 * * *'", cjs[0].Schedule)
	}
	if !cjs[0].Suspend {
		t.Error("Suspend = false, want true")
	}
}

func TestListNamespaces(t *testing.T) {
	client := &Client{Core: fake.NewSimpleClientset(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "production"},
		Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
	})}

	namespaces, err := client.ListNamespaces(context.Background())
	if err != nil {
		t.Fatalf("ListNamespaces returned error: %v", err)
	}
	if len(namespaces) != 1 {
		t.Fatalf("expected 1 namespace, got %d", len(namespaces))
	}
	if namespaces[0].Status != "Active" {
		t.Errorf("Status = %q, want Active", namespaces[0].Status)
	}
}
