/** SubTabs — a lightweight secondary tab strip used within dashboard sections. */
interface Props<K extends string> {
  tabs: { key: K; label: string }[];
  active: K;
  onChange: (key: K) => void;
}

export function SubTabs<K extends string>({ tabs, active, onChange }: Props<K>) {
  return (
    <div className="flex gap-1 mb-4 border-b border-pilot-border overflow-x-auto">
      {tabs.map((tab) => (
        <button
          key={tab.key}
          onClick={() => onChange(tab.key)}
          className={`px-3 py-2 text-sm font-medium border-b-2 whitespace-nowrap transition-colors ${
            active === tab.key
              ? "border-pilot-accent text-white"
              : "border-transparent text-pilot-muted hover:text-pilot-text-secondary"
          }`}
        >
          {tab.label}
        </button>
      ))}
    </div>
  );
}
