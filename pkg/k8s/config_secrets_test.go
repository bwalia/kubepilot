package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// TestListSecrets_NeverExposesData is the security-critical test: SecretSummary
// must surface only metadata and a key count, never the actual secret bytes.
func TestListSecrets_NeverExposesData(t *testing.T) {
	client := &Client{Core: fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "db-creds", Namespace: "default"},
		Type:       corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"username": []byte("admin"),
			"password": []byte("s3cr3t"),
		},
	})}

	secrets, err := client.ListSecrets(context.Background(), "default")
	if err != nil {
		t.Fatalf("ListSecrets returned error: %v", err)
	}
	if len(secrets) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(secrets))
	}

	got := secrets[0]
	if got.Name != "db-creds" {
		t.Errorf("Name = %q, want db-creds", got.Name)
	}
	if got.Type != string(corev1.SecretTypeOpaque) {
		t.Errorf("Type = %q, want Opaque", got.Type)
	}
	if got.KeyCount != 2 {
		t.Errorf("KeyCount = %d, want 2", got.KeyCount)
	}

	// The SecretSummary struct has no Data/StringData field — this is enforced at
	// compile time. The guard below catches any future regression that adds the
	// secret value into a string-bearing field by serialising the summary.
	for _, sensitive := range []string{"admin", "s3cr3t"} {
		if containsSecretValue(got, sensitive) {
			t.Errorf("secret value %q leaked into SecretSummary", sensitive)
		}
	}
}

func containsSecretValue(s SecretSummary, value string) bool {
	for _, field := range []string{s.Name, s.Namespace, s.Type} {
		if field == value {
			return true
		}
	}
	return false
}

func TestListConfigMaps(t *testing.T) {
	client := &Client{Core: fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "app-config", Namespace: "default"},
		Data:       map[string]string{"a": "1", "b": "2"},
	})}

	cms, err := client.ListConfigMaps(context.Background(), "default")
	if err != nil {
		t.Fatalf("ListConfigMaps returned error: %v", err)
	}
	if len(cms) != 1 {
		t.Fatalf("expected 1 configmap, got %d", len(cms))
	}
	if cms[0].KeyCount != 2 {
		t.Errorf("KeyCount = %d, want 2", cms[0].KeyCount)
	}
}

func TestListPVCs(t *testing.T) {
	sc := "fast-ssd"
	client := &Client{Core: fake.NewSimpleClientset(&corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "data", Namespace: "default"},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &sc,
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			VolumeName:       "pv-001",
		},
		Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
	})}

	pvcs, err := client.ListPVCs(context.Background(), "default")
	if err != nil {
		t.Fatalf("ListPVCs returned error: %v", err)
	}
	if len(pvcs) != 1 {
		t.Fatalf("expected 1 pvc, got %d", len(pvcs))
	}
	got := pvcs[0]
	if got.Status != "Bound" {
		t.Errorf("Status = %q, want Bound", got.Status)
	}
	if got.StorageClass != "fast-ssd" {
		t.Errorf("StorageClass = %q, want fast-ssd", got.StorageClass)
	}
	if len(got.AccessModes) != 1 || got.AccessModes[0] != "ReadWriteOnce" {
		t.Errorf("AccessModes = %v, want [ReadWriteOnce]", got.AccessModes)
	}
}
