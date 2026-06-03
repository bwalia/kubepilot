package k8s

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// ForwardTarget identifies the pod and remote port for a forwarding session.
type ForwardTarget struct {
	Namespace  string
	PodName    string
	RemotePort uint16
}

// ForwardResult is returned once the local listener is bound and ready.
type ForwardResult struct {
	LocalPort  uint16
	RemotePort uint16
	// Stop closes the forwarding session. Safe to call multiple times.
	Stop func()
	// ForwardErr receives exactly one value when ForwardPorts returns:
	// nil for a clean stop, or the error that terminated the session.
	ForwardErr <-chan error
}

// StartPodPortForward binds a local TCP listener (local port 0 → kernel-assigned)
// and proxies it to remotePort on the named pod, kubectl-style, against the target
// cluster. It returns once the listener is ready, or fails if readiness is not
// reached within readyTimeout.
func (c *Client) StartPodPortForward(ctx context.Context, target ForwardTarget, readyTimeout time.Duration) (*ForwardResult, error) {
	req := c.Core.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Namespace(target.Namespace).
		Name(target.PodName).
		SubResource("portforward")

	rt, upgrader, err := spdy.RoundTripperFor(c.RestConfig)
	if err != nil {
		return nil, fmt.Errorf("building spdy round tripper: %w", err)
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: rt}, http.MethodPost, req.URL())

	stopChan := make(chan struct{})
	readyChan := make(chan struct{})
	errChan := make(chan error, 1)

	portSpec := fmt.Sprintf("0:%d", target.RemotePort)
	pf, err := portforward.New(dialer, []string{portSpec}, stopChan, readyChan, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		return nil, fmt.Errorf("creating port forwarder: %w", err)
	}

	var stopOnce sync.Once
	stop := func() { stopOnce.Do(func() { close(stopChan) }) }

	go func() {
		// ForwardPorts blocks until stopChan is closed or the connection drops.
		errChan <- pf.ForwardPorts()
	}()

	select {
	case <-readyChan:
		// Listener is bound — safe to read the assigned local port.
	case err := <-errChan:
		return nil, fmt.Errorf("port-forward failed to start: %w", err)
	case <-time.After(readyTimeout):
		stop()
		return nil, fmt.Errorf("port-forward timed out waiting for listener (pod %s/%s may not be running)", target.Namespace, target.PodName)
	case <-ctx.Done():
		stop()
		return nil, ctx.Err()
	}

	ports, err := pf.GetPorts()
	if err != nil {
		stop()
		return nil, fmt.Errorf("reading forwarded ports: %w", err)
	}
	if len(ports) == 0 {
		stop()
		return nil, fmt.Errorf("port-forward reported no bound ports")
	}

	return &ForwardResult{
		LocalPort:  ports[0].Local,
		RemotePort: target.RemotePort,
		Stop:       stop,
		ForwardErr: errChan,
	}, nil
}

// ResolveServiceForwardTarget resolves a Service (and optional service port) to a
// concrete pod + container port suitable for port-forwarding. client-go can only
// forward to pods, so this picks the first ready endpoint pod, mirroring kubectl.
// A servicePort of 0 selects the Service's first port.
func (c *Client) ResolveServiceForwardTarget(ctx context.Context, namespace, serviceName string, servicePort int32) (*ForwardTarget, error) {
	svc, err := c.Core.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting service %s/%s: %w", namespace, serviceName, err)
	}
	if len(svc.Spec.Ports) == 0 {
		return nil, fmt.Errorf("service %s/%s exposes no ports", namespace, serviceName)
	}

	selected := svc.Spec.Ports[0]
	if servicePort != 0 {
		found := false
		for _, p := range svc.Spec.Ports {
			if p.Port == servicePort {
				selected = p
				found = true
				break
			}
		}
		if !found {
			available := make([]int32, 0, len(svc.Spec.Ports))
			for _, p := range svc.Spec.Ports {
				available = append(available, p.Port)
			}
			return nil, fmt.Errorf("service %s/%s has no port %d (available: %v)", namespace, serviceName, servicePort, available)
		}
	}

	// Find a ready endpoint pod backing the service.
	endpoints, err := c.Core.CoreV1().Endpoints(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting endpoints for service %s/%s: %w", namespace, serviceName, err)
	}

	podName := ""
	for _, subset := range endpoints.Subsets {
		for _, addr := range subset.Addresses {
			if addr.TargetRef != nil && addr.TargetRef.Kind == "Pod" {
				podName = addr.TargetRef.Name
				break
			}
		}
		if podName != "" {
			break
		}
	}
	if podName == "" {
		return nil, fmt.Errorf("service %s/%s has no ready endpoints to forward to", namespace, serviceName)
	}

	// Resolve the target port: numeric is used directly; a named port must be
	// resolved against the backing pod's container ports.
	remotePort := selected.TargetPort.IntValue()
	if selected.TargetPort.Type == intstr.String { // named port
		name := selected.TargetPort.StrVal
		pod, err := c.Core.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("getting pod %s/%s to resolve named port %q: %w", namespace, podName, name, err)
		}
		resolved := int32(0)
		for _, container := range pod.Spec.Containers {
			for _, cp := range container.Ports {
				if cp.Name == name {
					resolved = cp.ContainerPort
					break
				}
			}
			if resolved != 0 {
				break
			}
		}
		if resolved == 0 {
			return nil, fmt.Errorf("named port %q not found in pod %s/%s containers", name, namespace, podName)
		}
		remotePort = int(resolved)
	}

	if remotePort <= 0 {
		return nil, fmt.Errorf("could not resolve a target port for service %s/%s", namespace, serviceName)
	}

	return &ForwardTarget{
		Namespace:  namespace,
		PodName:    podName,
		RemotePort: uint16(remotePort),
	}, nil
}

// FirstPodContainerPort returns the first declared container port for a pod,
// used when starting a pod port-forward without an explicit remote port.
func (c *Client) FirstPodContainerPort(ctx context.Context, namespace, podName string) (uint16, error) {
	pod, err := c.Core.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("getting pod %s/%s: %w", namespace, podName, err)
	}
	for _, container := range pod.Spec.Containers {
		if len(container.Ports) > 0 {
			return uint16(container.Ports[0].ContainerPort), nil
		}
	}
	return 0, fmt.Errorf("pod %s/%s has no exposed container ports; specify a remote port explicitly", namespace, podName)
}
