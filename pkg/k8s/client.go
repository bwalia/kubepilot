// Package k8s provides Kubernetes API integration using client-go.
// It wraps cluster access into a unified Client type used by all KubePilot subsystems.
package k8s

import (
	"fmt"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned"
)

// Client aggregates all Kubernetes API clients used by KubePilot.
type Client struct {
	// Core is the standard typed Kubernetes client.
	Core kubernetes.Interface
	// Dynamic is used for CRD operations and unstructured resources.
	Dynamic dynamic.Interface
	// Metrics accesses the metrics-server API for pod/node resource usage.
	Metrics metricsv1beta1.Interface
	// RestConfig is the underlying rest config, exposed for operator-sdk use.
	RestConfig *rest.Config
}

// NewClient builds a Client from a kubeconfig path.
// If kubeconfigPath is empty it falls back to in-cluster configuration,
// which is the expected path when running inside a Kubernetes pod.
func NewClient(kubeconfigPath string) (*Client, error) {
	return NewClientWithContext(kubeconfigPath, "")
}

// NewClientWithContext builds a Client from a kubeconfig path using the
// specified context. If contextName is empty the current-context is used.
func NewClientWithContext(kubeconfigPath, contextName string) (*Client, error) {
	var (
		cfg *rest.Config
		err error
	)

	if kubeconfigPath != "" {
		rules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath}
		overrides := &clientcmd.ConfigOverrides{}
		if contextName != "" {
			overrides.CurrentContext = contextName
		}
		cfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	} else {
		cfg, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("building kubernetes rest config: %w", err)
	}

	// Raise the default QPS/Burst limits — KubePilot manages 10,000+ pods
	// and requires higher throughput than the conservative client-go defaults.
	cfg.QPS = 200
	cfg.Burst = 400

	coreClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("building core kubernetes client: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("building dynamic kubernetes client: %w", err)
	}

	metricsClient, err := metricsv1beta1.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("building metrics kubernetes client: %w", err)
	}

	return &Client{
		Core:       coreClient,
		Dynamic:    dynClient,
		Metrics:    metricsClient,
		RestConfig: cfg,
	}, nil
}

// ContextInfo describes a single context inside a kubeconfig file.
type ContextInfo struct {
	Name    string `json:"name"`
	Cluster string `json:"cluster"`
	User    string `json:"user"`
}

// ListContexts returns the contexts available in the given kubeconfig file
// along with the current-context name.
func ListContexts(kubeconfigPath string) (contexts []ContextInfo, currentContext string, err error) {
	if kubeconfigPath == "" {
		return nil, "", fmt.Errorf("kubeconfig path is required to list contexts")
	}

	rawCfg, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return nil, "", fmt.Errorf("loading kubeconfig: %w", err)
	}

	currentContext = rawCfg.CurrentContext
	for name, ctx := range rawCfg.Contexts {
		contexts = append(contexts, ContextInfo{
			Name:    name,
			Cluster: ctx.Cluster,
			User:    ctx.AuthInfo,
		})
	}
	return contexts, currentContext, nil
}
