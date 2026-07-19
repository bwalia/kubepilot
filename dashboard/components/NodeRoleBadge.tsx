interface NodeRoleFields {
  Roles?: string[];
  roles?: string[];
  ControlPlane?: boolean;
  control_plane?: boolean;
}

// Normalises the various role sources into a display list, defaulting to
// "worker" so a node always shows a definite type.
function nodeRoleList(node: NodeRoleFields): string[] {
  const roles = node.Roles ?? node.roles ?? [];
  if (roles.length > 0) return roles;
  return ["worker"];
}

function isControlPlane(node: NodeRoleFields, roles: string[]): boolean {
  if (node.ControlPlane ?? node.control_plane) return true;
  return roles.some((r) => r === "control-plane" || r === "master");
}

export function NodeRoleBadge({ node }: { node: NodeRoleFields }) {
  const roles = nodeRoleList(node);
  const controlPlane = isControlPlane(node, roles);

  // Control-plane nodes carry both "control-plane" and "master" labels; show a
  // single, clear badge for them and list any other roles alongside.
  const primary = controlPlane ? "control-plane" : roles[0];
  const extras = roles.filter((r) => r !== primary && r !== "control-plane" && r !== "master");

  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <span
        className={
          controlPlane
            ? "text-xs font-semibold px-2 py-0.5 rounded-md border bg-pilot-accent/15 text-pilot-accent border-pilot-accent/30"
            : "text-xs font-semibold px-2 py-0.5 rounded-md border bg-pilot-surface text-pilot-text-secondary border-pilot-border"
        }
      >
        {primary}
      </span>
      {extras.map((r) => (
        <span
          key={r}
          className="text-xs font-medium px-2 py-0.5 rounded-md border bg-pilot-surface text-pilot-muted border-pilot-border"
        >
          {r}
        </span>
      ))}
    </div>
  );
}
