package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

func TestResolveServiceForwardTarget_NumericPort(t *testing.T) {
	client := &Client{Core: fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{Port: 80, TargetPort: intstr.FromInt(8080)},
				},
			},
		},
		&corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
			Subsets: []corev1.EndpointSubset{{
				Addresses: []corev1.EndpointAddress{{
					IP:        "10.0.0.5",
					TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "web-abc"},
				}},
			}},
		},
	)}

	target, err := client.ResolveServiceForwardTarget(context.Background(), "default", "web", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target.PodName != "web-abc" {
		t.Errorf("PodName = %q, want web-abc", target.PodName)
	}
	if target.RemotePort != 8080 {
		t.Errorf("RemotePort = %d, want 8080", target.RemotePort)
	}
}

func TestResolveServiceForwardTarget_NamedPort(t *testing.T) {
	client := &Client{Core: fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{Port: 443, TargetPort: intstr.FromString("https")},
				},
			},
		},
		&corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
			Subsets: []corev1.EndpointSubset{{
				Addresses: []corev1.EndpointAddress{{
					TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "api-xyz"},
				}},
			}},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "api-xyz", Namespace: "default"},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "api",
					Ports: []corev1.ContainerPort{{Name: "https", ContainerPort: 8443}},
				}},
			},
		},
	)}

	target, err := client.ResolveServiceForwardTarget(context.Background(), "default", "api", 443)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target.RemotePort != 8443 {
		t.Errorf("RemotePort = %d, want 8443 (resolved from named port)", target.RemotePort)
	}
}

func TestResolveServiceForwardTarget_NoEndpoints(t *testing.T) {
	client := &Client{Core: fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Port: 80, TargetPort: intstr.FromInt(8080)}},
			},
		},
		&corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		},
	)}

	_, err := client.ResolveServiceForwardTarget(context.Background(), "default", "web", 0)
	if err == nil {
		t.Fatal("expected error for service with no ready endpoints, got nil")
	}
}

func TestResolveServiceForwardTarget_NamedPortNotFound(t *testing.T) {
	client := &Client{Core: fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Port: 443, TargetPort: intstr.FromString("nope")}},
			},
		},
		&corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
			Subsets: []corev1.EndpointSubset{{
				Addresses: []corev1.EndpointAddress{{
					TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "api-xyz"},
				}},
			}},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "api-xyz", Namespace: "default"},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "api",
					Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 8080}},
				}},
			},
		},
	)}

	_, err := client.ResolveServiceForwardTarget(context.Background(), "default", "api", 443)
	if err == nil {
		t.Fatal("expected error for unresolvable named port, got nil")
	}
}
