package security

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"go.uber.org/zap"
)

func newTestGuard() (*Guard, *fake.Clientset) {
	fakeClient := fake.NewSimpleClientset()
	log := zap.NewNop()
	guard := NewGuard(fakeClient, log)
	return guard, fakeClient
}

func createNamespace(t *testing.T, client *fake.Clientset) {
	t.Helper()
	_, err := client.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: CRCodeNamespace},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("creating namespace: %v", err)
	}
}

func TestSecretName(t *testing.T) {
	tests := []struct {
		changeID string
		expected string
	}{
		{"JIRA-1234", "kubepilot-crcode-jira-1234"},
		{"Simple", "kubepilot-crcode-simple"},
		{"has spaces!", "kubepilot-crcode-has-spaces-"},
	}

	for _, tc := range tests {
		got := secretName(tc.changeID)
		// Trim trailing dashes for comparison.
		expected := tc.expected
		if len(expected) > 0 && expected[len(expected)-1] == '-' {
			// The function trims trailing dashes.
			expected = expected[:len(expected)-1]
		}
		if got != expected {
			t.Errorf("secretName(%q) = %q, want %q", tc.changeID, got, expected)
		}
	}
}

func TestGuard_RegisterAndAuthorize(t *testing.T) {
	guard, client := newTestGuard()
	createNamespace(t, client)
	ctx := context.Background()

	err := guard.RegisterCRCode(ctx, "JIRA-100", "secret-code-123", nil)
	if err != nil {
		t.Fatalf("RegisterCRCode failed: %v", err)
	}

	// Authorize with correct code.
	err = guard.Authorize(ctx, "JIRA-100", "secret-code-123")
	if err != nil {
		t.Fatalf("Authorize should succeed with correct code: %v", err)
	}
}

func TestGuard_AuthorizeWrongCode(t *testing.T) {
	guard, client := newTestGuard()
	createNamespace(t, client)
	ctx := context.Background()

	_ = guard.RegisterCRCode(ctx, "JIRA-200", "correct-code", nil)

	err := guard.Authorize(ctx, "JIRA-200", "wrong-code")
	if err == nil {
		t.Fatal("Authorize should fail with wrong code")
	}
}

func TestGuard_AuthorizeEmptyCode(t *testing.T) {
	guard, _ := newTestGuard()
	ctx := context.Background()

	err := guard.Authorize(ctx, "JIRA-300", "")
	if err != ErrCRCodeRequired {
		t.Errorf("expected ErrCRCodeRequired, got %v", err)
	}
}

func TestGuard_AuthorizeNotRegistered(t *testing.T) {
	guard, client := newTestGuard()
	createNamespace(t, client)
	ctx := context.Background()

	err := guard.Authorize(ctx, "NONEXISTENT", "some-code")
	if err == nil {
		t.Fatal("Authorize should fail for unregistered change")
	}
}

func TestGuard_RegisterEmptyFields(t *testing.T) {
	guard, _ := newTestGuard()
	ctx := context.Background()

	err := guard.RegisterCRCode(ctx, "", "code", nil)
	if err == nil {
		t.Error("expected error for empty changeID")
	}

	err = guard.RegisterCRCode(ctx, "JIRA-1", "", nil)
	if err == nil {
		t.Error("expected error for empty crCode")
	}
}

func TestGuard_RegisterWithExpiry(t *testing.T) {
	guard, client := newTestGuard()
	createNamespace(t, client)
	ctx := context.Background()

	future := time.Now().Add(1 * time.Hour)
	err := guard.RegisterCRCode(ctx, "JIRA-400", "code-400", &future)
	if err != nil {
		t.Fatalf("RegisterCRCode with expiry failed: %v", err)
	}

	// Should work — not expired.
	err = guard.Authorize(ctx, "JIRA-400", "code-400")
	if err != nil {
		t.Fatalf("Authorize should succeed for non-expired code: %v", err)
	}
}

func TestGuard_ExpiredCode(t *testing.T) {
	guard, client := newTestGuard()
	createNamespace(t, client)
	ctx := context.Background()

	past := time.Now().Add(-1 * time.Hour)
	err := guard.RegisterCRCode(ctx, "JIRA-500", "code-500", &past)
	if err != nil {
		t.Fatalf("RegisterCRCode failed: %v", err)
	}

	err = guard.Authorize(ctx, "JIRA-500", "code-500")
	if err == nil {
		t.Fatal("Authorize should fail for expired code")
	}
}

func TestGuard_Revoke(t *testing.T) {
	guard, client := newTestGuard()
	createNamespace(t, client)
	ctx := context.Background()

	_ = guard.RegisterCRCode(ctx, "JIRA-600", "code-600", nil)

	err := guard.RevokeCRCode(ctx, "JIRA-600")
	if err != nil {
		t.Fatalf("RevokeCRCode failed: %v", err)
	}

	// Should fail after revocation.
	err = guard.Authorize(ctx, "JIRA-600", "code-600")
	if err == nil {
		t.Fatal("Authorize should fail after revocation")
	}
}

func TestGuard_RevokeNonexistent(t *testing.T) {
	guard, client := newTestGuard()
	createNamespace(t, client)
	ctx := context.Background()

	// Should not error for nonexistent secret.
	err := guard.RevokeCRCode(ctx, "NONEXISTENT")
	if err != nil {
		t.Fatalf("RevokeCRCode should not error for nonexistent: %v", err)
	}
}

func TestGuard_ReRegister(t *testing.T) {
	guard, client := newTestGuard()
	createNamespace(t, client)
	ctx := context.Background()

	_ = guard.RegisterCRCode(ctx, "JIRA-700", "old-code", nil)

	// Re-register with a new code.
	err := guard.RegisterCRCode(ctx, "JIRA-700", "new-code", nil)
	if err != nil {
		t.Fatalf("re-register failed: %v", err)
	}

	// Old code should no longer work.
	err = guard.Authorize(ctx, "JIRA-700", "old-code")
	if err == nil {
		t.Fatal("old code should no longer work after re-register")
	}

	// New code should work.
	err = guard.Authorize(ctx, "JIRA-700", "new-code")
	if err != nil {
		t.Fatalf("new code should work: %v", err)
	}
}
