package k8s

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RoleSummary is a concise view of a Kubernetes Role or ClusterRole.
type RoleSummary struct {
	Name      string       `json:"name"`
	Namespace string       `json:"namespace,omitempty"` // Empty for ClusterRoles.
	IsCluster bool         `json:"is_cluster"`
	Rules     []PolicyRule `json:"rules"`
}

// PolicyRule mirrors rbacv1.PolicyRule in a JSON-friendly format.
type PolicyRule struct {
	Verbs     []string `json:"verbs"`
	APIGroups []string `json:"api_groups"`
	Resources []string `json:"resources"`
}

// RoleBindingSummary describes who is bound to which role.
type RoleBindingSummary struct {
	Name      string          `json:"name"`
	Namespace string          `json:"namespace,omitempty"`
	IsCluster bool            `json:"is_cluster"`
	RoleRef   string          `json:"role_ref"`
	Subjects  []SubjectRef    `json:"subjects"`
}

// SubjectRef identifies a user, group, or service account in a binding.
type SubjectRef struct {
	Kind      string `json:"kind"` // User, Group, ServiceAccount
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// ListRoles returns all Roles in a namespace.
func (c *Client) ListRoles(ctx context.Context, namespace string) ([]RoleSummary, error) {
	list, err := c.Core.RbacV1().Roles(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing roles in namespace %q: %w", namespace, err)
	}

	summaries := make([]RoleSummary, 0, len(list.Items))
	for _, role := range list.Items {
		summaries = append(summaries, toRoleSummary(role.Name, role.Namespace, false, role.Rules))
	}
	return summaries, nil
}

// ListClusterRoles returns all ClusterRoles.
func (c *Client) ListClusterRoles(ctx context.Context) ([]RoleSummary, error) {
	list, err := c.Core.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing cluster roles: %w", err)
	}

	summaries := make([]RoleSummary, 0, len(list.Items))
	for _, role := range list.Items {
		summaries = append(summaries, toRoleSummary(role.Name, "", true, role.Rules))
	}
	return summaries, nil
}

// ListRoleBindings returns all RoleBindings in a namespace.
func (c *Client) ListRoleBindings(ctx context.Context, namespace string) ([]RoleBindingSummary, error) {
	list, err := c.Core.RbacV1().RoleBindings(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing role bindings in namespace %q: %w", namespace, err)
	}

	summaries := make([]RoleBindingSummary, 0, len(list.Items))
	for _, rb := range list.Items {
		summaries = append(summaries, toRoleBindingSummary(rb.Name, rb.Namespace, false, rb.RoleRef.Name, rb.Subjects))
	}
	return summaries, nil
}

// ListClusterRoleBindings returns all ClusterRoleBindings.
func (c *Client) ListClusterRoleBindings(ctx context.Context) ([]RoleBindingSummary, error) {
	list, err := c.Core.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing cluster role bindings: %w", err)
	}

	summaries := make([]RoleBindingSummary, 0, len(list.Items))
	for _, crb := range list.Items {
		summaries = append(summaries, toRoleBindingSummary(crb.Name, "", true, crb.RoleRef.Name, crb.Subjects))
	}
	return summaries, nil
}

func toRoleSummary(name, namespace string, isCluster bool, rules []rbacv1.PolicyRule) RoleSummary {
	policyRules := make([]PolicyRule, 0, len(rules))
	for _, r := range rules {
		policyRules = append(policyRules, PolicyRule{
			Verbs:     r.Verbs,
			APIGroups: r.APIGroups,
			Resources: r.Resources,
		})
	}
	return RoleSummary{
		Name:      name,
		Namespace: namespace,
		IsCluster: isCluster,
		Rules:     policyRules,
	}
}

func toRoleBindingSummary(name, namespace string, isCluster bool, roleRef string, subjects []rbacv1.Subject) RoleBindingSummary {
	subjectRefs := make([]SubjectRef, 0, len(subjects))
	for _, s := range subjects {
		subjectRefs = append(subjectRefs, SubjectRef{
			Kind:      s.Kind,
			Name:      s.Name,
			Namespace: s.Namespace,
		})
	}
	return RoleBindingSummary{
		Name:      name,
		Namespace: namespace,
		IsCluster: isCluster,
		RoleRef:   roleRef,
		Subjects:  subjectRefs,
	}
}
