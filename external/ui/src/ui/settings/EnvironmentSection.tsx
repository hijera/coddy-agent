import {
  useEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";
import {
  localFetch,
  setEnv,
  snapshotEnv,
  subscribeEnv,
} from "../env/remoteEnv";

type Remote = { name: string; url: string };

const CUSTOM = "__custom__";

/**
 * EnvironmentSection lets the user point this UI at a remote coddy http server (an
 * already-running instance) or back at the local one. The choice and its bearer token are
 * stored in this browser only (localStorage); configured remotes come from the local server's
 * httpserver.remotes. Connecting reloads the app so every request targets the chosen backend.
 */
export function EnvironmentSection() {
  const env = useSyncExternalStore(subscribeEnv, snapshotEnv, snapshotEnv);
  const [remotes, setRemotes] = useState<Remote[]>([]);
  const [remotesLoaded, setRemotesLoaded] = useState(false);

  const initialChoice = env.mode === "remote" ? env.baseUrl : "";
  const [choice, setChoice] = useState<string>(initialChoice);
  const [customUrl, setCustomUrl] = useState<string>(
    env.mode === "remote" ? env.baseUrl : "",
  );
  const [customName, setCustomName] = useState<string>(
    env.mode === "remote" ? (env.name ?? "") : "",
  );
  const [token, setToken] = useState<string>(
    env.mode === "remote" ? env.token : "",
  );

  useEffect(() => {
    let alive = true;
    // Always read the LOCAL config for the offered remotes, regardless of the active environment.
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
        /* configured remotes are optional; the user can enter a custom URL */
      })
      .finally(() => {
        if (alive) setRemotesLoaded(true);
      });
    return () => {
      alive = false;
    };
  }, []);

  const knownUrls = useMemo(
    () => new Set(remotes.map((r) => r.url)),
    [remotes],
  );
  // One-time normalization once the configured remotes have loaded: if the active remote is not in
  // the list, reflect it as the Custom option (pre-filled). Guarded by a ref so it runs exactly
  // once and never fights the user's own selection — in particular, choosing Local while connected
  // to a custom remote must stick, so the user can roam Local <-> Remote freely.
  const normalizedRef = useRef(false);
  useEffect(() => {
    if (normalizedRef.current || !remotesLoaded) return;
    normalizedRef.current = true;
    if (env.mode === "remote" && !knownUrls.has(env.baseUrl)) {
      setCustomUrl(env.baseUrl);
      setCustomName(env.name ?? "");
      setChoice(CUSTOM);
    }
  }, [remotesLoaded, env, knownUrls]);

  const isRemoteChoice = choice !== "";
  const targetUrl = choice === CUSTOM ? customUrl.trim() : choice;

  const switchToLocal = () => {
    setEnv({ mode: "local" });
    window.location.reload();
  };

  const connect = () => {
    if (!isRemoteChoice) {
      setEnv({ mode: "local" });
    } else {
      const url = targetUrl;
      if (!url) return;
      const name =
        choice === CUSTOM
          ? customName.trim() || url
          : remotes.find((r) => r.url === url)?.name || url;
      setEnv({ mode: "remote", baseUrl: url, token: token.trim(), name });
    }
    // Reload so all in-flight state (sessions, models, config) re-fetches from the new backend.
    window.location.reload();
  };

  return (
    <div className="settings-group" data-testid="environment-section">
      <p className="settings-field-desc">
        Point this UI at a remote, already-running <code>coddy http</code>{" "}
        server, or use the local one. The connection and its token are stored in
        this browser only. The remote must allow this origin via{" "}
        <code>httpserver.cors</code>.
      </p>

      <p className="settings-ok" data-testid="environment-current">
        {env.mode === "local"
          ? "Connected to: Local (this origin)"
          : `Connected to: ${env.name ? env.name + " — " : ""}${env.baseUrl}`}
      </p>

      {env.mode === "remote" ? (
        <div className="settings-row">
          <button
            type="button"
            className="settings-btn"
            onClick={switchToLocal}
            data-testid="environment-use-local"
          >
            ← Switch back to Local
          </button>
        </div>
      ) : null}

      <div className="settings-row">
        <label className="settings-label">
          <input
            type="radio"
            name="coddy-env"
            checked={!isRemoteChoice}
            onChange={() => setChoice("")}
          />{" "}
          Local (this origin)
        </label>
        {remotes.map((r) => (
          <label className="settings-label" key={r.url}>
            <input
              type="radio"
              name="coddy-env"
              checked={choice === r.url}
              onChange={() => setChoice(r.url)}
            />{" "}
            {r.name || r.url} <span className="settings-muted">— {r.url}</span>
          </label>
        ))}
        <label className="settings-label">
          <input
            type="radio"
            name="coddy-env"
            checked={choice === CUSTOM}
            onChange={() => setChoice(CUSTOM)}
          />{" "}
          Custom remote…
        </label>
      </div>

      {choice === CUSTOM ? (
        <>
          <div className="settings-row">
            <span className="settings-label">Name</span>
            <input
              className="settings-input"
              type="text"
              placeholder="prod box"
              value={customName}
              onChange={(e) => setCustomName(e.target.value)}
            />
          </div>
          <div className="settings-row">
            <span className="settings-label">Base URL</span>
            <input
              className="settings-input"
              type="text"
              placeholder="https://box.example:12345"
              value={customUrl}
              onChange={(e) => setCustomUrl(e.target.value)}
              data-testid="environment-custom-url"
            />
          </div>
        </>
      ) : null}

      {isRemoteChoice ? (
        <div className="settings-row">
          <span className="settings-label">Bearer token</span>
          <p className="settings-field-desc">
            The remote's <code>auth_token</code>. Leave empty if the remote has
            no auth. Stored in this browser only.
          </p>
          <input
            className="settings-input"
            type="password"
            autoComplete="off"
            placeholder="token"
            value={token}
            onChange={(e) => setToken(e.target.value)}
            data-testid="environment-token"
          />
        </div>
      ) : null}

      <div className="settings-row">
        <button
          type="button"
          className="settings-btn settings-btn-primary"
          onClick={connect}
          disabled={isRemoteChoice && !targetUrl}
          data-testid="environment-connect"
        >
          {isRemoteChoice ? "Connect to remote" : "Use Local"}
        </button>
      </div>
    </div>
  );
}
