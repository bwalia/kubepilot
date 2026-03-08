// Package operator implements the KubePilot Kubernetes operator.
// It reconciles KubePilotCluster, KubePilotJob, and KubePilotCRCode CRDs,
// manages agent lifecycle, and enforces CR code validation for production jobs.
package operator

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	kubepilotv1 "github.com/kubepilot/kubepilot/api/v1alpha1"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kubepilotv1.AddToScheme(scheme))
}

// Config holds operator configuration.
type Config struct {
	MetricsAddr     string
	HealthProbeAddr string
	LeaderElect     bool
	KubeConfig      string
}

// Manager wraps the controller-runtime manager.
type Manager struct {
	inner ctrl.Manager
	log   *zap.Logger
}

// NewManager creates and configures the operator manager with all CRD controllers registered.
func NewManager(cfg Config, log *zap.Logger) (*Manager, error) {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: cfg.MetricsAddr,
		},
		HealthProbeBindAddress: cfg.HealthProbeAddr,
		LeaderElection:         cfg.LeaderElect,
		LeaderElectionID:       "kubepilot-operator-leader",
	})
	if err != nil {
		return nil, fmt.Errorf("creating controller-runtime manager: %w", err)
	}

	// Register CRD reconcilers.
	if err := (&ClusterReconciler{Client: mgr.GetClient(), Log: log}).SetupWithManager(mgr); err != nil {
		return nil, fmt.Errorf("setting up KubePilotCluster reconciler: %w", err)
	}
	if err := (&JobReconciler{Client: mgr.GetClient(), Log: log}).SetupWithManager(mgr); err != nil {
		return nil, fmt.Errorf("setting up KubePilotJob reconciler: %w", err)
	}
	if err := (&CRCodeReconciler{Client: mgr.GetClient(), Log: log}).SetupWithManager(mgr); err != nil {
		return nil, fmt.Errorf("setting up KubePilotCRCode reconciler: %w", err)
	}

	// Health check endpoints for liveness/readiness probes.
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return nil, fmt.Errorf("adding healthz check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return nil, fmt.Errorf("adding readyz check: %w", err)
	}

	return &Manager{inner: mgr, log: log}, nil
}

// Start runs the operator until ctx is cancelled.
func (m *Manager) Start(ctx context.Context) error {
	m.log.Info("Starting KubePilot operator")
	return m.inner.Start(ctx)
}

// ─────────────────────────────────────────
// KubePilotCluster Reconciler
// ─────────────────────────────────────────

// ClusterReconciler reconciles KubePilotCluster resources.
type ClusterReconciler struct {
	client.Client
	Log *zap.Logger
}

// +kubebuilder:rbac:groups=kubepilot.io,resources=kubepilotclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubepilot.io,resources=kubepilotclusters/status,verbs=get;update;patch

func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var cluster kubepilotv1.KubePilotCluster
	if err := r.Get(ctx, req.NamespacedName, &cluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	r.Log.Info("Reconciling KubePilotCluster", zap.String("name", cluster.Name))

	// TODO: Deploy/update MCP agent DaemonSet in the target cluster.
	// TODO: Update cluster.Status.Connected based on agent heartbeat.

	return ctrl.Result{}, nil
}

func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubepilotv1.KubePilotCluster{}).
		Complete(r)
}

// ─────────────────────────────────────────
// KubePilotJob Reconciler
// ─────────────────────────────────────────

// JobReconciler reconciles KubePilotJob resources.
type JobReconciler struct {
	client.Client
	Log *zap.Logger
}

// +kubebuilder:rbac:groups=kubepilot.io,resources=kubepilotjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubepilot.io,resources=kubepilotjobs/status,verbs=get;update;patch

func (r *JobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var job kubepilotv1.KubePilotJob
	if err := r.Get(ctx, req.NamespacedName, &job); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	r.Log.Info("Reconciling KubePilotJob",
		zap.String("name", job.Name),
		zap.String("env", job.Spec.TargetEnvironment),
	)

	// Production jobs must be blocked until a valid CR code is registered.
	if job.Spec.TargetEnvironment == "production" && job.Spec.ChangeIDRef == "" {
		r.Log.Warn("Production job missing ChangeIDRef — blocking execution",
			zap.String("job", job.Name),
		)
		job.Status.Phase = "Blocked"
		_ = r.Status().Update(ctx, &job)
		return ctrl.Result{}, nil
	}

	// TODO: Submit to the KubePilot job scheduler via internal API.

	return ctrl.Result{}, nil
}

func (r *JobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubepilotv1.KubePilotJob{}).
		Complete(r)
}

// ─────────────────────────────────────────
// KubePilotCRCode Reconciler
// ─────────────────────────────────────────

// CRCodeReconciler reconciles KubePilotCRCode resources.
type CRCodeReconciler struct {
	client.Client
	Log *zap.Logger
}

// +kubebuilder:rbac:groups=kubepilot.io,resources=kubepilotcrcodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubepilot.io,resources=kubepilotcrcodes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *CRCodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var crcode kubepilotv1.KubePilotCRCode
	if err := r.Get(ctx, req.NamespacedName, &crcode); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	r.Log.Info("Reconciling KubePilotCRCode", zap.String("change_id", crcode.Spec.ChangeID))

	// Mark the CR code as active if it has not expired.
	active := true
	if crcode.Spec.ExpiresAt != nil && crcode.Spec.ExpiresAt.Time.Before(crcode.CreationTimestamp.Time) {
		active = false
	}

	crcode.Status.Active = active
	if err := r.Status().Update(ctx, &crcode); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating CR code status: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *CRCodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubepilotv1.KubePilotCRCode{}).
		Complete(r)
}
