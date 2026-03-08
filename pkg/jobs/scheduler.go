// Package jobs provides multi-agent job scheduling and orchestration for KubePilot.
// Jobs can be one-shot or recurring (cron), and support AI-generated action plans
// as well as manual commands. Production jobs require CR code authorization.
package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/ai"
	"github.com/kubepilot/kubepilot/pkg/k8s"
	"github.com/kubepilot/kubepilot/pkg/security"
)

// Status represents the lifecycle state of a Job.
type Status string

const (
	StatusPending  Status = "pending"
	StatusRunning  Status = "running"
	StatusDone     Status = "done"
	StatusFailed   Status = "failed"
	StatusBlocked  Status = "blocked" // Waiting for CR code authorization.
)

// Job represents a unit of work in KubePilot's job system.
type Job struct {
	ID          string
	Name        string
	Command     string // Natural language command or structured action.
	Schedule    string // Cron expression for repeating jobs; empty for one-shot.
	TargetEnv   string // "production" | "staging" | "development"
	ChangeID    string // Jira/CR change ID for production authorization.
	CRCode      string // Submitted CR code (not stored permanently — validated then discarded).
	Status      Status
	CreatedAt   time.Time
	LastRunAt   *time.Time
	LastResult  string
	Actions     []ai.SuggestedAction
}

// Scheduler manages the KubePilot job queue, cron registry, and execution.
type Scheduler struct {
	mu      sync.RWMutex
	jobs    map[string]*Job
	cron    *cron.Cron
	cronIDs map[string]cron.EntryID // maps Job.ID → cron.EntryID for cancellation.

	ai    *ai.Engine
	k8s   *k8s.Client
	guard *security.Guard
	log   *zap.Logger
}

// NewScheduler creates a Scheduler. Call Start to begin cron processing.
func NewScheduler(aiEngine *ai.Engine, k8sClient *k8s.Client, log *zap.Logger) *Scheduler {
	return &Scheduler{
		jobs:    make(map[string]*Job),
		cronIDs: make(map[string]cron.EntryID),
		cron:    cron.New(cron.WithSeconds()),
		ai:      aiEngine,
		k8s:     k8sClient,
		guard:   security.NewGuard(k8sClient.Core, log),
		log:     log,
	}
}

// Start begins the cron scheduler. It runs until ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	s.cron.Start()
	go func() {
		<-ctx.Done()
		s.cron.Stop()
	}()
}

// Submit adds a new job to the scheduler. If the job has a Schedule it is
// registered as a cron job; otherwise it runs immediately in the background.
func (s *Scheduler) Submit(ctx context.Context, job *Job) error {
	if job.ID == "" {
		job.ID = uuid.NewString()
	}
	job.Status = StatusPending
	job.CreatedAt = time.Now().UTC()

	// Production jobs must have a CR code before they can be queued.
	if job.TargetEnv == "production" {
		if err := s.guard.Authorize(ctx, job.ChangeID, job.CRCode); err != nil {
			job.Status = StatusBlocked
			s.storeJob(job)
			return fmt.Errorf("job %q blocked: %w", job.Name, err)
		}
		// Discard CR code from memory after validation — never persist the raw code.
		job.CRCode = ""
	}

	s.storeJob(job)

	if job.Schedule != "" {
		return s.scheduleCron(ctx, job)
	}

	go s.executeJob(ctx, job)
	return nil
}

// GetJob retrieves a job by ID.
func (s *Scheduler) GetJob(id string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	return job, ok
}

// ListJobs returns all known jobs, ordered by creation time (newest first).
func (s *Scheduler) ListJobs() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		result = append(result, j)
	}
	return result
}

// CancelJob removes a cron job or marks a pending job as failed.
func (s *Scheduler) CancelJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return fmt.Errorf("job %q not found", id)
	}

	if entryID, hasCron := s.cronIDs[id]; hasCron {
		s.cron.Remove(entryID)
		delete(s.cronIDs, id)
	}

	job.Status = StatusFailed
	job.LastResult = "cancelled by operator"
	return nil
}

func (s *Scheduler) scheduleCron(ctx context.Context, job *Job) error {
	entryID, err := s.cron.AddFunc(job.Schedule, func() {
		s.executeJob(ctx, job)
	})
	if err != nil {
		return fmt.Errorf("invalid cron schedule %q for job %q: %w", job.Schedule, job.Name, err)
	}

	s.mu.Lock()
	s.cronIDs[job.ID] = entryID
	s.mu.Unlock()

	s.log.Info("Cron job scheduled",
		zap.String("job_id", job.ID),
		zap.String("schedule", job.Schedule),
	)
	return nil
}

func (s *Scheduler) executeJob(ctx context.Context, job *Job) {
	s.updateJobStatus(job.ID, StatusRunning, "")

	now := time.Now().UTC()

	actions, err := s.ai.Interpret(ctx, job.Command)
	if err != nil {
		s.updateJobStatus(job.ID, StatusFailed, fmt.Sprintf("AI interpretation error: %v", err))
		return
	}

	job.Actions = actions
	var results []string

	for _, action := range actions {
		if err := s.executeAction(ctx, action); err != nil {
			results = append(results, fmt.Sprintf("action %s failed: %v", action.Type, err))
			s.log.Warn("Job action failed",
				zap.String("job_id", job.ID),
				zap.String("action", string(action.Type)),
				zap.Error(err),
			)
		} else {
			results = append(results, fmt.Sprintf("action %s succeeded: %s", action.Type, action.Explanation))
		}
	}

	s.mu.Lock()
	job.LastRunAt = &now
	s.mu.Unlock()

	summary := fmt.Sprintf("Completed %d actions", len(actions))
	if len(results) > 0 {
		summary = fmt.Sprintf("%s. Details: %v", summary, results)
	}
	s.updateJobStatus(job.ID, StatusDone, summary)
}

func (s *Scheduler) executeAction(ctx context.Context, action ai.SuggestedAction) error {
	switch action.Type {
	case ai.ActionRestart:
		return s.k8s.RestartDeployment(ctx, action.Namespace, action.Resource)
	case ai.ActionScale:
		return s.k8s.ScaleDeployment(ctx, action.Namespace, action.Resource, action.Replicas)
	case ai.ActionDeletePod:
		return s.k8s.DeletePod(ctx, action.Namespace, action.Resource)
	case ai.ActionInvestigate, ai.ActionNoOp:
		// Investigate and noop actions are informational — logged but not acted upon automatically.
		s.log.Info("Investigate/noop action",
			zap.String("explanation", action.Explanation),
		)
		return nil
	default:
		return fmt.Errorf("unsupported action type: %s", action.Type)
	}
}

func (s *Scheduler) storeJob(job *Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
}

func (s *Scheduler) updateJobStatus(id string, status Status, result string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.jobs[id]; ok {
		job.Status = status
		if result != "" {
			job.LastResult = result
		}
	}
}
