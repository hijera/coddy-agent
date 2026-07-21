import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import { createPortal } from "react-dom";
import {
  connectLocal,
  connectRemote,
  getRemoteToken,
  hasRemoteToken,
  localFetch,
  snapshotEnv,
  subscribeEnv,
} from "../env/remoteEnv";
import {
  serverSnapshotShellStack,
  snapshotShellStack,
  subscribeShellStack,
} from "../shellBreakpoint";

type Remote = { name: string; url: string };

function hostLabel(url: string): string {
  return url.replace(/^https?:\/\//, "");
}

function normUrl(url: string): string {
  return url.trim().replace(/\/+$/, "");
}

/**
 * EnvironmentChip is the composer-bar environment selector (Claude-Code style): a chip that opens
 * a menu to point the UI at the local server or a remote coddy http, without leaving the composer.
 * The choice and per-remote bearer tokens are stored in this browser only; connecting reloads so
 * every request targets the chosen backend.
 */
export function EnvironmentChip() {
  const env = useSyncExternalStore(subscribeEnv, snapshotEnv, snapshotEnv);
  const isMobileShell = useSyncExternalStore(
    subscribeShellStack,
    snapshotShellStack,
    serverSnapshotShellStack,
  );
  const [remotes, setRemotes] = useState<Remote[]>([]);
  const [open, setOpen] = useState(false);
  const [anchor, setAnchor] = useState<DOMRect | null>(null);
  const [pending, setPending] = useState<{ url: string; name: string } | null>(
    null,
  );
  const [pendingToken, setPendingToken] = useState("");
  const [adding, setAdding] = useState(false);
  const [addName, setAddName] = useState("");
  const [addUrl, setAddUrl] = useState("");
  const [addToken, setAddToken] = useState("");
  const btnRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    let alive = true;
    localFetch("/coddy/config")
      .then((r) => (r.ok ? r.json() : null))
      .then((cfg) => {
        if (!alive || !cfg) return;
        const list = cfg?.httpserver?.remotes;
        if (Array.isArray(list)) {
          setRemotes(
            list
              .map((r: unknown) => {
                const o = (r ?? {}) as Record<string, unknown>;
                return { name: String(o.name ?? ""), url: String(o.url ?? "") };
              })
              .filter((r: Remote) => r.url.trim() !== ""),
          );
        }
      })
      .catch(() => {
        /* configured remotes are optional; Add remote… still works */
      });
    return () => {
      alive = false;
    };
  }, []);

  const label =
    env.mode === "local" ? "Local" : env.name || hostLabel(env.baseUrl);

  const openMenu = () => {
    if (btnRef.current) setAnchor(btnRef.current.getBoundingClientRect());
    setPending(null);
    setAdding(false);
    setOpen(true);
  };
  const closeMenu = () => {
    setOpen(false);
    setPending(null);
    setAdding(false);
  };

  const chooseRemote = (r: Remote) => {
    if (hasRemoteToken(r.url)) {
      connectRemote(r.url, getRemoteToken(r.url), r.name);
      return;
    }
    setAdding(false);
    setPendingToken("");
    setPending({ url: r.url, name: r.name });
  };

  const useSheet = isMobileShell;

  return (
    <div className="mode">
      <button
        ref={btnRef}
        type="button"
        className="composer-tab mode-btn mode-env"
        aria-label="Environment"
        title="Environment (local or remote coddy http)"
        aria-haspopup="menu"
        aria-expanded={open}
        data-testid="composer-env-btn"
        onClick={() => (open ? closeMenu() : openMenu())}
      >
        <span
          className="mode-env-dot"
          aria-hidden="true"
          data-remote={env.mode === "remote" ? "1" : undefined}
        />
        {label}
      </button>
      {open && (useSheet || anchor)
        ? createPortal(
            <>
              <button
                type="button"
                className={`mode-menu-backdrop ${useSheet ? "mode-menu-backdrop--scrim" : ""}`}
                aria-hidden="true"
                tabIndex={-1}
                onMouseDown={(e) => {
                  e.preventDefault();
                  closeMenu();
                }}
              />
              <div
                className={`mode-menu mode-menu--env ${useSheet ? "mode-menu--sheet" : "mode-menu--portal opens-up"}`}
                role="menu"
                data-testid="composer-env-menu"
                style={
                  useSheet || !anchor
                    ? undefined
                    : {
                        left: anchor.left,
                        bottom: window.innerHeight - anchor.top + 8,
                      }
                }
                onKeyDown={(e) => {
                  if (e.key === "Escape") {
                    e.preventDefault();
                    closeMenu();
                  }
                }}
              >
                <div className="mode-menu-group-label">Environment</div>
                <button
                  type="button"
                  role="menuitem"
                  className={`mode-item ${env.mode === "local" ? "is-selected" : ""}`}
                  data-testid="composer-env-local"
                  onClick={() => connectLocal()}
                >
                  Local (this origin)
                </button>

                {remotes.length ? (
                  <div className="mode-menu-group-label">Remote</div>
                ) : null}
                {remotes.map((r) => {
                  const active =
                    env.mode === "remote" && env.baseUrl === normUrl(r.url);
                  return (
                    <button
                      key={r.url}
                      type="button"
                      role="menuitem"
                      className={`mode-item ${active ? "is-selected" : ""}`}
                      title={r.url}
                      onClick={() => chooseRemote(r)}
                    >
                      {r.name || hostLabel(r.url)}
                      <span className="mode-env-sub">— {hostLabel(r.url)}</span>
                    </button>
                  );
                })}

                {pending ? (
                  <div className="mode-menu-form">
                    <div className="mode-menu-form-title">
                      Token for {pending.name || hostLabel(pending.url)}
                    </div>
                    <input
                      className="mode-menu-filter"
                      type="password"
                      autoComplete="off"
                      autoFocus
                      placeholder="bearer token (empty if none)"
                      value={pendingToken}
                      onChange={(e) => setPendingToken(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") {
                          e.preventDefault();
                          connectRemote(
                            pending.url,
                            pendingToken.trim(),
                            pending.name,
                          );
                        }
                      }}
                    />
                    <div className="mode-menu-form-actions">
                      <button
                        type="button"
                        className="mode-item"
                        onClick={() =>
                          connectRemote(
                            pending.url,
                            pendingToken.trim(),
                            pending.name,
                          )
                        }
                      >
                        Connect
                      </button>
                      <button
                        type="button"
                        className="mode-item"
                        onClick={() => setPending(null)}
                      >
                        Cancel
                      </button>
                    </div>
                  </div>
                ) : adding ? (
                  <div className="mode-menu-form">
                    <div className="mode-menu-form-title">Add a remote</div>
                    <input
                      className="mode-menu-filter"
                      type="text"
                      placeholder="name"
                      value={addName}
                      onChange={(e) => setAddName(e.target.value)}
                    />
                    <input
                      className="mode-menu-filter"
                      type="text"
                      placeholder="https://box.example:12345"
                      value={addUrl}
                      data-testid="composer-env-add-url"
                      onChange={(e) => setAddUrl(e.target.value)}
                    />
                    <input
                      className="mode-menu-filter"
                      type="password"
                      autoComplete="off"
                      placeholder="bearer token (empty if none)"
                      value={addToken}
                      onChange={(e) => setAddToken(e.target.value)}
                    />
                    <div className="mode-menu-form-actions">
                      <button
                        type="button"
                        className="mode-item"
                        disabled={!addUrl.trim()}
                        onClick={() =>
                          connectRemote(
                            addUrl.trim(),
                            addToken.trim(),
                            addName.trim() || addUrl.trim(),
                          )
                        }
                      >
                        Connect
                      </button>
                      <button
                        type="button"
                        className="mode-item"
                        onClick={() => setAdding(false)}
                      >
                        Cancel
                      </button>
                    </div>
                  </div>
                ) : (
                  <button
                    type="button"
                    role="menuitem"
                    className="mode-item mode-env-add"
                    data-testid="composer-env-add"
                    onClick={() => {
                      setPending(null);
                      setAdding(true);
                    }}
                  >
                    + Add remote…
                  </button>
                )}
              </div>
            </>,
            document.body,
          )
        : null}
    </div>
  );
}
