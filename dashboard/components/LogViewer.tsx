/**
 * LogViewer — Structured log viewer with error highlighting.
 * Displays evidence data from RCA reports with syntax highlighting for errors.
 */

interface LogViewerProps {
  title: string;
  content: string;
  maxHeight?: string;
}

const ERROR_PATTERNS = [
  /error/i,
  /fatal/i,
  /panic/i,
  /exception/i,
  /fail/i,
  /oom/i,
  /killed/i,
  /timeout/i,
  /refused/i,
  /denied/i,
];

const WARNING_PATTERNS = [/warn/i, /deprecated/i, /retry/i, /backoff/i];

function classifyLine(line: string): "error" | "warning" | "normal" {
  for (const pattern of ERROR_PATTERNS) {
    if (pattern.test(line)) return "error";
  }
  for (const pattern of WARNING_PATTERNS) {
    if (pattern.test(line)) return "warning";
  }
  return "normal";
}

const LINE_COLORS = {
  error: "text-pilot-danger bg-pilot-danger/10",
  warning: "text-pilot-warning bg-pilot-warning/10",
  normal: "text-pilot-muted",
};

export function LogViewer({ title, content, maxHeight = "400px" }: LogViewerProps) {
  const lines = content.split("\n");

  return (
    <div className="bg-pilot-surface border border-pilot-border rounded-lg overflow-hidden shadow-card">
      <div className="px-3 py-2 border-b border-pilot-border flex items-center justify-between">
        <span className="eyebrow">{title}</span>
        <span className="text-xs text-pilot-muted tabular-nums">{lines.length} lines</span>
      </div>
      <div
        className="overflow-y-auto font-mono text-xs p-2"
        style={{ maxHeight }}
      >
        {lines.map((line, i) => {
          const cls = classifyLine(line);
          return (
            <div key={i} className={`flex gap-2 px-1 ${LINE_COLORS[cls]}`}>
              <span className="text-pilot-muted select-none w-8 text-right shrink-0">
                {i + 1}
              </span>
              <span className="whitespace-pre-wrap break-all">{line}</span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
