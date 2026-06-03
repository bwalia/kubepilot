package k8s

import (
	"context"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func newYAMLTestClient(objects ...runtime.Object) *Client {
	scheme := runtime.NewScheme()
	return &Client{Dynamic: dynamicfake.NewSimpleDynamicClient(scheme, objects...)}
}

// TestGetResourceYAML_RedactsSecretData asserts that Secret data and stringData
// blocks are stripped, and that managedFields are removed.
func TestGetResourceYAML_RedactsSecretData(t *testing.T) {
	secret := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"name":          "db-creds",
			"namespace":     "default",
			"managedFields": []interface{}{map[string]interface{}{"manager": "kubectl"}},
		},
		"type": "Opaque",
		"data": map[string]interface{}{
			"password": "czNjcjN0",
		},
		"stringData": map[string]interface{}{
			"username": "admin",
		},
	}}

	client := newYAMLTestClient(secret)

	out, err := client.GetResourceYAML(context.Background(), "secret", "default", "db-creds")
	if err != nil {
		t.Fatalf("GetResourceYAML returned error: %v", err)
	}

	for _, forbidden := range []string{"czNjcjN0", "password", "stringData", "admin", "managedFields"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("rendered YAML must not contain %q, got:\n%s", forbidden, out)
		}
	}
	// Non-sensitive metadata should still be present.
	if !strings.Contains(out, "db-creds") {
		t.Errorf("expected resource name in YAML, got:\n%s", out)
	}
}

func TestGetResourceYAML_StripsManagedFieldsForNonSecret(t *testing.T) {
	cm := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":          "app-config",
			"namespace":     "default",
			"managedFields": []interface{}{map[string]interface{}{"manager": "kubectl"}},
		},
		"data": map[string]interface{}{"key": "value"},
	}}

	client := newYAMLTestClient(cm)

	out, err := client.GetResourceYAML(context.Background(), "configmap", "default", "app-config")
	if err != nil {
		t.Fatalf("GetResourceYAML returned error: %v", err)
	}
	if strings.Contains(out, "managedFields") {
		t.Errorf("managedFields should be stripped, got:\n%s", out)
	}
	// ConfigMap data is not sensitive — it should be retained.
	if !strings.Contains(out, "value") {
		t.Errorf("expected configmap data in YAML, got:\n%s", out)
	}
}

func TestGetResourceYAML_UnsupportedKind(t *testing.T) {
	client := newYAMLTestClient()
	_, err := client.GetResourceYAML(context.Background(), "frobnicator", "default", "x")
	if err == nil {
		t.Fatal("expected error for unsupported kind, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported resource kind") {
		t.Errorf("unexpected error message: %v", err)
	}
}
