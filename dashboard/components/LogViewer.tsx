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
  error: "text-red-400 bg-red-900/20",
  warning: "text-yellow-400 bg-yellow-900/10",
  normal: "text-pilot-muted",
};

export function LogViewer({ title, content, maxHeight = "400px" }: LogViewerProps) {
  const lines = content.split("\n");

  return (
    <div className="bg-pilot-surface border border-pilot-border rounded overflow-hidden">
      <div className="px-3 py-2 border-b border-pilot-border flex items-center justify-between">
        <span className="text-xs font-bold text-pilot-accent uppercase">{title}</span>
        <span className="text-xs text-pilot-muted">{lines.length} lines</span>
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
