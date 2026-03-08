// Package security implements production protection for KubePilot.
// All production-impacting Kubernetes operations MUST pass through CR code
// validation before executing. CR codes are stored as Kubernetes Secrets
// in a dedicated namespace so they can be injected by external workflows
// (Jira agents, CI/CD pipelines, GitOps) without being embedded in source code.
package security

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"go.uber.org/zap"
)

// invalidCharsRe matches characters not allowed in Kubernetes resource names.
var invalidCharsRe = regexp.MustCompile(`[^a-z0-9\-.]`)

const (
	// CRCodeNamespace is the dedicated namespace for CR code secrets.
	// Isolating secrets here allows tight RBAC: only the KubePilot service
	// account and authorized injection pipelines have access.
	CRCodeNamespace = "kubepilot-security"

	// CRCodeSecretPrefix is prepended to the secret name for every CR code.
	// Example secret name: kubepilot-crcode-JIRA-1234
	CRCodeSecretPrefix = "kubepilot-crcode-"

	// CRCodeKey is the key within the secret's Data map that holds the code.
	CRCodeKey = "cr-code"

	// CRCodeExpiryKey holds an optional RFC3339 expiry timestamp.
	CRCodeExpiryKey = "expires-at"
)

// secretName converts a change ID into a valid Kubernetes secret name.
// Kubernetes names must be lowercase RFC 1123 subdomains.
func secretName(changeID string) string {
	name := CRCodeSecretPrefix + strings.ToLower(changeID)
	name = invalidCharsRe.ReplaceAllString(name, "-")
	// Trim trailing dashes/dots.
	name = strings.TrimRight(name, "-.")
	return name
}

// ErrCRCodeRequired is returned when a production operation is attempted
// without providing a CR code. This is a distinct sentinel error so callers
// can present a specific "authorization required" message in the UI.
var ErrCRCodeRequired = errors.New("production change requires a valid CR code")

// ErrCRCodeInvalid is returned when the provided CR code does not match
// the value stored in the Kubernetes secret.
var ErrCRCodeInvalid = errors.New("CR code is invalid or expired")

// Guard is the production safety gate. Instantiate one per KubePilot server
// and call Authorize before any production-impacting action.
type Guard struct {
	k8s kubernetes.Interface
	log *zap.Logger
}

// NewGuard creates a Guard using the provided Kubernetes client.
func NewGuard(k8s kubernetes.Interface, log *zap.Logger) *Guard {
	return &Guard{k8s: k8s, log: log}
}

// Authorize validates that the provided crCode matches the Kubernetes-stored
// secret for the given changeID. It returns nil on success or a descriptive
// error if authorization fails.
//
// Usage pattern:
//
//	if err := guard.Authorize(ctx, "JIRA-1234", submittedCode); err != nil {
//	    return err // propagate to dashboard / CLI
//	}
//	// Safe to execute production change here.
func (g *Guard) Authorize(ctx context.Context, changeID, crCode string) error {
	if crCode == "" {
		return ErrCRCodeRequired
	}

	secretName := secretName(changeID)
	secret, err := g.k8s.CoreV1().Secrets(CRCodeNamespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			g.log.Warn("CR code secret not found", zap.String("secret", secretName))
			return fmt.Errorf("%w: no CR code registered for change %q", ErrCRCodeInvalid, changeID)
		}
		return fmt.Errorf("reading CR code secret %q: %w", secretName, err)
	}

	// Check expiry before comparing codes to avoid timing-dependent leaks.
	if err := checkExpiry(secret); err != nil {
		return err
	}

	storedEncoded, ok := secret.Data[CRCodeKey]
	if !ok {
		return fmt.Errorf("CR code secret %q missing key %q", secretName, CRCodeKey)
	}

	// Decode the base64-encoded stored value (Kubernetes stores secret data as base64).
	stored, err := base64.StdEncoding.DecodeString(string(storedEncoded))
	if err != nil {
		// The secret value may already be decoded by client-go — try direct comparison.
		stored = storedEncoded
	}

	// Use constant-time comparison to prevent timing attacks.
	if subtle.ConstantTimeCompare([]byte(crCode), stored) != 1 {
		g.log.Warn("CR code mismatch",
			zap.String("change_id", changeID),
			zap.String("secret", secretName),
		)
		return ErrCRCodeInvalid
	}

	g.log.Info("CR code authorized",
		zap.String("change_id", changeID),
		zap.String("secret", secretName),
	)
	return nil
}

// RegisterCRCode stores a new CR code as a Kubernetes secret.
// This is called by external workflows (Jira agent, CI/CD) to inject codes.
// The secret is created in CRCodeNamespace with an optional expiry.
func (g *Guard) RegisterCRCode(ctx context.Context, changeID, crCode string, expiresAt *time.Time) error {
	if changeID == "" || crCode == "" {
		return fmt.Errorf("changeID and crCode must not be empty")
	}

	secretData := map[string][]byte{
		CRCodeKey: []byte(crCode),
	}

	if expiresAt != nil {
		secretData[CRCodeExpiryKey] = []byte(expiresAt.UTC().Format(time.RFC3339))
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName(changeID),
			Namespace: CRCodeNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "kubepilot",
				"kubepilot.io/change-id":       changeID,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: secretData,
	}

	_, err := g.k8s.CoreV1().Secrets(CRCodeNamespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			// Update the existing secret — supports re-registration of codes.
			existing, getErr := g.k8s.CoreV1().Secrets(CRCodeNamespace).Get(ctx, secret.Name, metav1.GetOptions{})
			if getErr != nil {
				return fmt.Errorf("getting existing CR code secret: %w", getErr)
			}
			existing.Data = secretData
			_, err = g.k8s.CoreV1().Secrets(CRCodeNamespace).Update(ctx, existing, metav1.UpdateOptions{})
		}
	}
	if err != nil {
		return fmt.Errorf("registering CR code secret for change %q: %w", changeID, err)
	}

	g.log.Info("CR code registered", zap.String("change_id", changeID))
	return nil
}

// RevokeCRCode deletes the CR code secret, preventing further use.
// Call this after a change has been applied or abandoned.
func (g *Guard) RevokeCRCode(ctx context.Context, changeID string) error {
	secretName := secretName(changeID)
	err := g.k8s.CoreV1().Secrets(CRCodeNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("revoking CR code secret %q: %w", secretName, err)
	}
	g.log.Info("CR code revoked", zap.String("change_id", changeID))
	return nil
}

// checkExpiry returns an error if the secret has an expiry timestamp that has passed.
func checkExpiry(secret *corev1.Secret) error {
	expiryBytes, ok := secret.Data[CRCodeExpiryKey]
	if !ok {
		// No expiry set — code does not expire.
		return nil
	}

	expiresAt, err := time.Parse(time.RFC3339, string(expiryBytes))
	if err != nil {
		// Malformed expiry is treated as expired to fail securely.
		return fmt.Errorf("%w: malformed expiry timestamp in CR code secret", ErrCRCodeInvalid)
	}

	if time.Now().UTC().After(expiresAt) {
		return fmt.Errorf("%w: CR code expired at %s", ErrCRCodeInvalid, expiresAt.Format(time.RFC3339))
	}
	return nil
}
