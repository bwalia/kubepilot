package dashboard

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/kubepilot/kubepilot/pkg/k8s"
)

func TestTopPodUsageUnionsEachMetric(t *testing.T) {
	pods := []k8s.PodUsage{
		{Name: "cpu-hog", CPUUsageMilli: 900},
		{Name: "mem-hog", MemUsageBytes: 9 << 30},
		{Name: "disk-hog", StorageUsedBytes: 90 << 30},
		{Name: "idle"},
	}
	got := topPodUsage(pods, 1)
	if len(got) != 3 {
		t.Fatalf("want the 3 hogs, got %+v", got)
	}
	for _, p := range got {
		if p.Name == "idle" {
			t.Fatal("idle pod should not be in the top 1 of any metric")
		}
	}
}

func TestPodUsageCacheServesStaleWhileRefreshing(t *testing.T) {
	cs := fake.NewSimpleClientset(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "ns"}})
	client := &k8s.Client{Core: cs}
	var c podUsageCache

	first, err := c.get(context.Background(), client, "")
	if err != nil || len(first.Pods) != 1 {
		t.Fatalf("first get: %v %+v", err, first)
	}

	// Age the entry past fresh and add a pod: the next call must return the
	// cached report immediately and refresh in the background.
	key := podUsageKey{client, ""}
	c.mu.Lock()
	c.entries[key].fetched = time.Now().Add(-podUsageFresh - time.Second)
	c.mu.Unlock()
	_, _ = cs.CoreV1().Pods("ns").Create(context.Background(),
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "b", Namespace: "ns"}}, metav1.CreateOptions{})

	stale, _ := c.get(context.Background(), client, "")
	if stale != first {
		t.Fatal("stale entry should be served as is")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if r, _ := c.get(context.Background(), client, ""); len(r.Pods) == 2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("background refresh never replaced the stale report")
}
