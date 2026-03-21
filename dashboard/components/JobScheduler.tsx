/**
 * JobScheduler — Jira-style job management panel.
 * Operators can create, view, and cancel KubePilot jobs from this panel.
 */
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  listJobs,
  submitJob,
  cancelJob,
  type Job,
  type SubmitJobRequest,
} from "@/lib/api";
import {
  Play,
  Square,
  Clock,
  CheckCircle,
  XCircle,
  AlertTriangle,
  Plus,
  RefreshCw,
} from "lucide-react";
import clsx from "clsx";

const STATUS_CONFIG: Record<
  Job["Status"],
  { label: string; color: string; Icon: React.ElementType }
> = {
  pending: { label: "Pending", color: "text-pilot-muted", Icon: Clock },
  running: { label: "Running", color: "text-pilot-accent", Icon: RefreshCw },
  done: { label: "Done", color: "text-pilot-success", Icon: CheckCircle },
  failed: { label: "Failed", color: "text-pilot-danger", Icon: XCircle },
  blocked: { label: "Blocked \u2013 CR Required", color: "text-pilot-warning", Icon: AlertTriangle },
};

export function JobScheduler() {
  const [showForm, setShowForm] = useState(false);
  const qc = useQueryClient();

  const { data: jobs = [], isLoading } = useQuery({
    queryKey: ["jobs"],
    queryFn: listJobs,
    refetchInterval: 5_000,
  });

  const cancelMut = useMutation({
    mutationFn: cancelJob,
    onSuccess: () => qc.invalidateQueries({ queryKey: ["jobs"] }),
  });

  return (
    <section>
      <div className="flex items-center justify-between mb-5">
        <h2 className="text-base font-bold text-white">Jobs</h2>
        <button
          onClick={() => setShowForm(true)}
          className="flex items-center gap-2 bg-pilot-accent text-white text-sm px-4 py-2 rounded-lg hover:bg-blue-500 font-semibold shadow-glow-blue"
        >
          <Plus className="w-4 h-4" />
          New Job
        </button>
      </div>

      {isLoading ? (
        <div className="space-y-3">
          {[...Array(4)].map((_, i) => (
            <div key={i} className="h-16 bg-pilot-surface rounded-xl animate-pulse" />
          ))}
        </div>
      ) : (
        <div className="space-y-2">
          {jobs.length === 0 && (
            <div className="text-center text-pilot-muted text-sm py-12 border border-dashed border-pilot-border rounded-xl bg-pilot-surface/50">
              No jobs yet. Create one to start AI-driven automation.
            </div>
          )}
          {jobs.map((job) => (
            <JobCard
              key={job.ID}
              job={job}
              onCancel={() => cancelMut.mutate(job.ID)}
            />
          ))}
        </div>
      )}

      {showForm && (
        <NewJobModal
          onClose={() => setShowForm(false)}
          onCreated={() => {
            setShowForm(false);
            qc.invalidateQueries({ queryKey: ["jobs"] });
          }}
        />
      )}
    </section>
  );
}

function JobCard({ job, onCancel }: { job: Job; onCancel: () => void }) {
  const cfg = STATUS_CONFIG[job.Status] ?? STATUS_CONFIG.pending;
  const Icon = cfg.Icon;

  return (
    <div className="bg-pilot-surface border border-pilot-border rounded-xl px-5 py-4 flex items-start justify-between gap-4 hover:border-pilot-border-hover transition-colors shadow-card">
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 mb-1">
          <span className="text-sm font-bold text-white truncate">{job.Name}</span>
          {job.Schedule && (
            <span className="text-xs bg-pilot-surface-2 text-pilot-muted px-2 py-0.5 rounded-md font-medium">
              <Clock className="inline w-3 h-3 mr-1" />
              {job.Schedule}
            </span>
          )}
          <span
            className={clsx(
              "text-xs px-2 py-0.5 rounded-md font-semibold",
              job.TargetEnv === "production"
                ? "bg-red-900/30 text-pilot-danger"
                : job.TargetEnv === "staging"
                ? "bg-yellow-900/30 text-pilot-warning"
                : "bg-green-900/30 text-pilot-success"
            )}
          >
            {job.TargetEnv}
          </span>
        </div>
        <p className="text-sm text-pilot-text-secondary truncate">{job.Command}</p>
        {job.LastResult && (
          <p className="text-sm text-pilot-muted mt-1 italic truncate">{job.LastResult}</p>
        )}
      </div>

      <div className="flex items-center gap-3 shrink-0">
        <span className={clsx("flex items-center gap-1.5 text-sm font-medium", cfg.color)}>
          <Icon className={clsx("w-4 h-4", job.Status === "running" && "animate-spin")} />
          {cfg.label}
        </span>
        {(job.Status === "pending" || job.Status === "running") && (
          <button
            onClick={onCancel}
            title="Cancel job"
            className="text-pilot-muted hover:text-pilot-danger p-1"
          >
            <Square className="w-4 h-4" />
          </button>
        )}
      </div>
    </div>
  );
}

function NewJobModal({
  onClose,
  onCreated,
}: {
  onClose: () => void;
  onCreated: () => void;
}) {
  const [form, setForm] = useState<SubmitJobRequest>({
    name: "",
    command: "",
    schedule: "",
    target_environment: "development",
    change_id: "",
    cr_code: "",
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const set = (key: keyof SubmitJobRequest) => (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>) =>
    setForm((f) => ({ ...f, [key]: e.target.value }));

  const handleSubmit = async () => {
    if (!form.name || !form.command) {
      setError("Name and command are required.");
      return;
    }
    setLoading(true);
    setError(null);
    try {
      await submitJob(form);
      onCreated();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to create job.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div className="bg-pilot-bg border border-pilot-border rounded-2xl w-full max-w-lg p-6 animate-fade-in shadow-card-hover">
        <div className="flex items-center justify-between mb-6">
          <h3 className="font-bold text-white text-lg flex items-center gap-2">
            <Play className="w-5 h-5 text-pilot-accent" />
            Create New Job
          </h3>
          <button onClick={onClose} className="text-pilot-muted hover:text-white p-1">
            <XCircle className="w-5 h-5" />
          </button>
        </div>

        <div className="space-y-4">
          <Field label="Job Name" required>
            <input type="text" value={form.name} onChange={set("name")} placeholder="e.g. fix-production-crashers" className={inputCls} />
          </Field>

          <Field label="AI Command" required>
            <textarea
              value={form.command}
              onChange={set("command")}
              rows={3}
              placeholder='e.g. "Fix all CrashLoopBackOff pods in the production namespace"'
              className={inputCls + " resize-none"}
            />
          </Field>

          <div className="grid grid-cols-2 gap-4">
            <Field label="Environment">
              <select value={form.target_environment} onChange={set("target_environment")} className={inputCls}>
                <option value="development">development</option>
                <option value="staging">staging</option>
                <option value="production">production</option>
              </select>
            </Field>
            <Field label="Cron Schedule (optional)">
              <input type="text" value={form.schedule} onChange={set("schedule")} placeholder="0 */6 * * *" className={inputCls} />
            </Field>
          </div>

          {form.target_environment === "production" && (
            <div className="bg-red-950/50 border border-pilot-danger/40 rounded-xl p-4 space-y-3">
              <p className="text-sm text-pilot-danger font-bold flex items-center gap-1.5">
                <AlertTriangle className="w-4 h-4" />
                Production job \u2014 CR code required
              </p>
              <Field label="Change Request ID">
                <input type="text" value={form.change_id ?? ""} onChange={set("change_id")} placeholder="INFRA-1234" className={inputCls} />
              </Field>
              <Field label="CR Code">
                <input type="password" value={form.cr_code ?? ""} onChange={set("cr_code")} placeholder="Enter CR code" className={inputCls} />
              </Field>
            </div>
          )}

          {error && (
            <p className="text-sm text-pilot-danger bg-red-950/50 border border-pilot-danger/40 rounded-xl p-3">{error}</p>
          )}
        </div>

        <div className="flex gap-3 mt-6">
          <button onClick={onClose} className="flex-1 border border-pilot-border text-white text-sm py-2.5 rounded-lg font-medium hover:bg-pilot-surface">
            Cancel
          </button>
          <button
            onClick={handleSubmit}
            disabled={loading}
            className="flex-1 bg-pilot-accent text-white text-sm py-2.5 rounded-lg font-bold hover:bg-blue-500 disabled:opacity-50 shadow-glow-blue"
          >
            {loading ? "Submitting\u2026" : "Create Job"}
          </button>
        </div>
      </div>
    </div>
  );
}

function Field({ label, required, children }: { label: string; required?: boolean; children: React.ReactNode }) {
  return (
    <div>
      <label className="block text-sm text-pilot-muted mb-1.5 font-medium">
        {label}
        {required && <span className="text-pilot-danger ml-0.5">*</span>}
      </label>
      {children}
    </div>
  );
}

const inputCls =
  "w-full bg-pilot-surface border border-pilot-border rounded-lg px-4 py-2.5 text-sm text-white placeholder:text-pilot-muted focus:outline-none focus:border-pilot-accent focus:ring-1 focus:ring-pilot-accent/30";
