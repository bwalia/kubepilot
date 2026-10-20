package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestFormatCPUMilli(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"1", "1000m"},
		{"1000m", "1000m"},
		{"50m", "50m"},
		{"250m", "250m"},
		{"2", "2000m"},
	}
	for _, tc := range cases {
		q := resource.MustParse(tc.in)
		if got := FormatCPUMilli(q); got != tc.want {
			t.Errorf("FormatCPUMilli(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestSumContainerResources_NormalizesCPUAndSums(t *testing.T) {
	rm := sumContainerResources([]corev1.Container{
		{
			Name: "njs",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("50m"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
				Limits: corev1.ResourceList{
					// Bare core count — the bug that made RCA claim "usage 999m > limit 1m".
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
		},
		{
			Name: "sidecar",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("50m")},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")},
			},
		},
	})

	if rm.CPURequest != "100m" {
		t.Errorf("CPURequest=%q want 100m", rm.CPURequest)
	}
	if rm.CPULimit != "1500m" {
		t.Errorf("CPULimit=%q want 1500m (1000m+500m)", rm.CPULimit)
	}
	if rm.MemLimit != "256Mi" {
		t.Errorf("MemLimit=%q want 256Mi", rm.MemLimit)
	}
}

func TestUsagePercent_CPUWithinLimit(t *testing.T) {
	pct := usagePercent("999m", "1000m")
	if pct == nil {
		t.Fatal("expected percent")
	}
	if *pct < 99 || *pct > 100 {
		t.Errorf("got %.2f want ~99.9", *pct)
	}

	idle := usagePercent("1m", "1000m")
	if idle == nil || *idle > 1 {
		t.Errorf("idle percent=%v want ~0.1", idle)
	}

	// The misread case: bare "1" as limit must parse as 1 core, not 1m.
	bare := usagePercent("999m", "1")
	if bare == nil {
		t.Fatal("expected percent for usage=999m limit=1")
	}
	if *bare > 100 {
		t.Errorf("999m of 1 core should be <100%%, got %.2f — unit confusion still present", *bare)
	}
}
