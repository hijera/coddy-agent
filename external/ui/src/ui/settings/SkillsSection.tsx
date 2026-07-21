import { useCallback, useEffect, useState } from "react";
import { SchemaForm, IconTrash, type JsonSchema } from "./SchemaForm";

type InstalledSkill = {
  name: string;
  description: string;
  file_path: string;
  enabled: boolean;
  version?: string;
  source?: string;
};

type SkillUpdate = {
  name: string;
  source: string;
  version: string;
  latest: string;
  update_available: boolean;
};

async function fetchInstalled(): Promise<InstalledSkill[]> {
  const res = await fetch("/coddy/skills");
  if (!res.ok) return [];
  const data = (await res.json()) as { items?: InstalledSkill[] };
  return data.items ?? [];
}

async function fetchUpdates(): Promise<SkillUpdate[]> {
  const res = await fetch("/coddy/skills/updates");
  if (!res.ok) return [];
  const data = (await res.json()) as { items?: SkillUpdate[] };
  return data.items ?? [];
}

async function apiSend(
  path: string,
  method: "POST" | "DELETE",
  body?: unknown,
): Promise<{ ok: boolean; error?: string }> {
  const init: RequestInit = { method };
  if (body !== undefined) {
    init.headers = { "Content-Type": "application/json" };
    init.body = JSON.stringify(body);
  }
  const res = await fetch(path, init);
  if (!res.ok) {
    try {
      const j = (await res.json()) as { error?: { message?: string } };
      return { ok: false, error: j.error?.message || `HTTP ${res.status}` };
    } catch {
      return { ok: false, error: `HTTP ${res.status}` };
    }
  }
  return { ok: true };
}

function IconPlug() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none"
      stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <path d="M7 22H4a2 2 0 0 1-2-2v-3a2 2 0 0 0-2 0V7a2 2 0 0 0 2 0H7" />
      <path d="M15 7h4a2 2 0 0 1 2 2v4a2 2 0 0 0 0 2v3a2 2 0 0 1-2 2h-3" />
      <line x1="12" y1="2" x2="12" y2="22" />
    </svg>
  );
}

function IconSync() {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none"
      stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <path d="M21 2v6h-6" />
      <path d="M3 12a9 9 0 0 1 15-6.7L21 8" />
      <path d="M3 22v-6h6" />
      <path d="M21 12a9 9 0 0 1-15 6.7L3 16" />
    </svg>
  );
}

function IconUpdate() {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none"
      stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <path d="M12 3v12" />
      <polyline points="7 10 12 15 17 10" />
      <path d="M5 21h14" />
    </svg>
  );
}

/**
 * SkillsSection is the combined Skills tab: the schema-driven `skills.dirs` /
 * `skills.sources` editor (adding, listing, and removing remote sources is the
 * generic array editor rendered by SchemaForm — no duplicate control here),
 * a Sync action that fetches the configured sources, and the installed-skills
 * list with versions, enable/disable, delete, and a per-skill Update action
 * shown when a newer version is available upstream.
 */
export function SkillsSection(props: {
  schema: JsonSchema;
  value: Record<string, unknown>;
  onChange: (next: Record<string, unknown>) => void;
}) {
  const { schema, value, onChange } = props;
  const [installed, setInstalled] = useState<InstalledSkill[]>([]);
  const [updates, setUpdates] = useState<Record<string, SkillUpdate>>({});
  const [busy, setBusy] = useState<Record<string, boolean>>({});
  const [error, setError] = useState<string | null>(null);
  const [status, setStatus] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [syncing, setSyncing] = useState(false);

  const loadInstalled = useCallback(async () => {
    setLoading(true);
    setInstalled(await fetchInstalled());
    setLoading(false);
  }, []);

  const refreshUpdates = useCallback(async () => {
    const ups = await fetchUpdates();
    const map: Record<string, SkillUpdate> = {};
    for (const u of ups) map[u.name] = u;
    setUpdates(map);
    return map;
  }, []);

  useEffect(() => {
    void loadInstalled();
  }, [loadInstalled]);

  const onToggle = (skill: InstalledSkill) => {
    setBusy((p) => ({ ...p, [skill.name]: true }));
    setError(null);
    void (async () => {
      const action = skill.enabled ? "disable" : "enable";
      const res = await apiSend(`/coddy/skills/${encodeURIComponent(skill.name)}/${action}`, "POST");
      if (!res.ok) {
        setError(res.error || `Failed to ${action}`);
      } else {
        await loadInstalled();
      }
      setBusy((p) => ({ ...p, [skill.name]: false }));
    })();
  };

  const onRemove = (skill: InstalledSkill) => {
    setBusy((p) => ({ ...p, [skill.name]: true }));
    setError(null);
    void (async () => {
      const res = await apiSend(`/coddy/skills/${encodeURIComponent(skill.name)}`, "DELETE");
      if (!res.ok) {
        setError(res.error || "Failed to delete");
      } else {
        await loadInstalled();
      }
      setBusy((p) => ({ ...p, [skill.name]: false }));
    })();
  };

  const onUpdateSkill = (skill: InstalledSkill) => {
    setBusy((p) => ({ ...p, [skill.name]: true }));
    setError(null);
    setStatus(null);
    void (async () => {
      const res = await apiSend(`/coddy/skills/${encodeURIComponent(skill.name)}/update`, "POST");
      if (!res.ok) {
        setError(res.error || "Update failed");
      } else {
        setStatus(`Updated ${skill.name}.`);
        await loadInstalled();
        await refreshUpdates();
      }
      setBusy((p) => ({ ...p, [skill.name]: false }));
    })();
  };

  // One action: fetch every configured source in skills.sources, then refresh
  // the installed list and re-check versions. (Save the settings first so newly
  // added sources are persisted before syncing.)
  const onSync = () => {
    setSyncing(true);
    setError(null);
    setStatus(null);
    void (async () => {
      const res = await apiSend("/coddy/skills/sync", "POST");
      if (!res.ok) setError(res.error || "Sync failed");
      else {
        setStatus("Sync complete.");
        await loadInstalled();
        await refreshUpdates();
      }
      setSyncing(false);
    })();
  };

  return (
    <div className="settings-skills-section">
      <SchemaForm schema={schema} value={value} onChange={onChange} />

      <div className="skills-sources-actions">
        <button
          type="button"
          className="settings-btn settings-btn-icon skills-sync-btn"
          disabled={syncing || loading}
          onClick={onSync}
          title="Sync — fetch the remote skill sources and refresh the list"
          aria-label="Sync remote skill sources"
          data-testid="skills-sync"
        >
          <IconSync />
        </button>
      </div>

      <p className="appearance-section-label settings-skills-installed-label">
        Installed skills
      </p>
      <p className="settings-field-desc">
        You can also install skills via <code>npx skills</code> or <code>npx skillsbd</code> — they
        land in <code>~/.agents/skills/</code> and are picked up automatically.
      </p>
      {error ? <p className="settings-error">{error}</p> : null}
      {status ? <p className="settings-muted">{status}</p> : null}

      {loading ? (
        <p className="settings-muted">Loading…</p>
      ) : installed.length === 0 ? (
        <p className="settings-muted">
          No skills found. Use <code>npx skills</code> or <code>npx skillsbd</code> to install.
        </p>
      ) : (
        <ul className="skills-list">
          {installed.map((sk) => {
            const upd = updates[sk.name];
            const hasUpdate = !!upd?.update_available;
            return (
              <li
                key={sk.name}
                className={`skills-list-item${sk.enabled ? "" : " is-disabled"}`}
              >
                <IconPlug />
                <div className="skills-list-item-text">
                  <div className="skills-list-item-name">
                    {sk.name}
                    {sk.version ? (
                      <span className="skills-list-item-version">v{sk.version}</span>
                    ) : null}
                    {sk.source ? (
                      <span className="skills-list-item-badge" title={`Synced from ${sk.source}`}>
                        remote
                      </span>
                    ) : null}
                  </div>
                  {sk.description ? (
                    <div className="skills-list-item-desc">{sk.description}</div>
                  ) : null}
                </div>
                {hasUpdate ? (
                  <button
                    type="button"
                    className="settings-btn settings-btn-icon settings-btn-primary skills-update-btn"
                    disabled={!!busy[sk.name]}
                    onClick={() => onUpdateSkill(sk)}
                    title={`Update ${sk.name} from v${upd?.version || sk.version || "?"} to v${upd?.latest}`}
                    aria-label={`Update ${sk.name} to version ${upd?.latest}`}
                    data-testid={`skills-update-${sk.name}`}
                  >
                    <IconUpdate />
                  </button>
                ) : null}
                <button
                  type="button"
                  role="switch"
                  aria-checked={sk.enabled}
                  className="skill-switch"
                  disabled={!!busy[sk.name]}
                  onClick={() => onToggle(sk)}
                  title={sk.enabled ? "Enabled — click to disable" : "Disabled — click to enable"}
                  aria-label={`${sk.enabled ? "Disable" : "Enable"} ${sk.name}`}
                  data-testid={`skills-toggle-${sk.name}`}
                >
                  <span className="skill-switch-thumb" />
                </button>
                {sk.source ? (
                  <button
                    type="button"
                    className="settings-btn settings-btn-icon settings-btn-danger"
                    disabled={!!busy[sk.name]}
                    onClick={() => onRemove(sk)}
                    title="Delete"
                    aria-label={`Delete ${sk.name}`}
                    data-testid={`skills-delete-${sk.name}`}
                  >
                    <IconTrash />
                  </button>
                ) : null}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
