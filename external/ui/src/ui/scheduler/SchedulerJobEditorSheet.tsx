import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import {
  schedulerCreateJob,
  schedulerDeleteJob,
  schedulerGetJob,
  schedulerPatchJob,
  schedulerPauseJob,
  schedulerResumeJob,
} from "./api";
import { describeCronScheduleOrError } from "./cronDescribe";
import { MarkdownLineEditor } from "./MarkdownLineEditor";
import { setSchedulerJobHash, setSchedulerListHash } from "./hashRoute";
import type { SchedulerJob, SchedulerJobCreate } from "./types";

type EditorMode = "create" | "edit";

type FieldErrors = Partial<{
  jobId: string;
  description: string;
  schedule: string;
  body: string;
}>;

function validateJobId(raw: string): string | null {
  const s = raw.trim();
  if (!s) {
    return "Required";
  }
  if (s.length > 64) {
    return "Too long";
  }
  if (/\s/.test(s)) {
    return "No spaces - use hyphens (example: daily-report)";
  }
  // Filename basename: English letters, digits, and hyphens only.
  if (!/^[A-Za-z0-9][A-Za-z0-9-]*$/.test(s)) {
    return "Only letters, digits, and hyphens (example: daily-report)";
  }
  return null;
}

export function SchedulerJobEditorSheet(props: {
  open: boolean;
  mode: EditorMode;
  jobId: string | null;
  availableModels: string[];
  defaultModel: string;
  currentCwd: string;
  onClose: () => void;
  /** Pass new job id after create so parent can switch to edit without relying on hashchange. */
  onSaved: (createdJobId?: string) => void;
  onDeleted: () => void;
}) {
  const [loadErr, setLoadErr] = useState<string | null>(null);
  const [saveErr, setSaveErr] = useState<string | null>(null);
  const [fieldErrs, setFieldErrs] = useState<FieldErrors>({});
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  const [jobIdField, setJobIdField] = useState("");
  const [description, setDescription] = useState("");
  const [schedule, setSchedule] = useState("0 * * * *");
  const [cwd, setCwd] = useState("");
  const [model, setModel] = useState("");
  const [modeField, setModeField] = useState("agent");
  const [body, setBody] = useState("");
  const [paused, setPaused] = useState(false);

  useEffect(() => {
    if (!props.open) {
      return;
    }
    setSaveErr(null);
    setFieldErrs({});
    setLoadErr(null);
    if (props.mode === "create") {
      setJobIdField("");
      setDescription("");
      setSchedule("0 * * * *");
      setCwd(props.currentCwd || "");
      setModel(props.defaultModel || "");
      setModeField("agent");
      setBody("");
      setPaused(false);
      setLoading(false);
      return;
    }
    const jid = (props.jobId || "").trim();
    if (!jid) {
      return;
    }
    let cancelled = false;
    setLoading(true);
    void (async () => {
      const res = await schedulerGetJob(jid);
      if (cancelled) {
        return;
      }
      setLoading(false);
      if (!res.ok) {
        setLoadErr(res.message);
        return;
      }
      const j: SchedulerJob = res.data;
      setJobIdField(j.job_id);
      setDescription(j.description || "");
      setSchedule(j.schedule || "");
      setCwd(j.cwd || "");
      setModel(j.model || "");
      setModeField((j.mode || "agent").toLowerCase() === "plan" ? "plan" : "agent");
      setBody(j.body || "");
      setPaused(!!j.paused);
    })();
    return () => {
      cancelled = true;
    };
  }, [props.open, props.mode, props.jobId]);

  if (!props.open || typeof document === "undefined") {
    return null;
  }

  const cronHint = describeCronScheduleOrError(schedule);

  async function onSave() {
    setSaveErr(null);
    const errs: FieldErrors = {};
    const jid = jobIdField.trim();
    const desc = description.trim();
    const sch = schedule.trim();
    const bod = body;
    const jidErr = validateJobId(jid);
    if (jidErr) errs.jobId = jidErr;
    if (!desc) errs.description = "Required";
    if (!sch) errs.schedule = "Required";
    if (!bod.trim()) errs.body = "Required";
    setFieldErrs(errs);
    if (Object.keys(errs).length > 0) {
      return;
    }
    setSaving(true);
    try {
      if (props.mode === "create") {
        const payload: SchedulerJobCreate = {
          job_id: jid,
          description: desc,
          schedule: sch,
          body: bod,
          paused,
          ...(cwd.trim() ? { cwd: cwd.trim() } : {}),
          ...(model.trim() ? { model: model.trim() } : {}),
          ...(modeField ? { mode: modeField } : {}),
        };
        const res = await schedulerCreateJob(payload);
        if (!res.ok) {
          setSaveErr(res.message);
          return;
        }
        setSchedulerJobHash(jid);
        props.onSaved(jid);
        return;
      }
      const existing = (props.jobId || "").trim();
      if (!existing) {
        setSaveErr("Missing job id");
        return;
      }
      const res = await schedulerPatchJob(existing, {
        description: desc,
        schedule: sch,
        body: bod,
        paused,
        ...(cwd.trim() ? { cwd: cwd.trim() } : { cwd: "" }),
        ...(model.trim() ? { model: model.trim() } : { model: "" }),
        mode: modeField,
      });
      if (!res.ok) {
        setSaveErr(res.message);
        return;
      }
      props.onSaved();
    } finally {
      setSaving(false);
    }
  }

  async function onPauseToggle() {
    const jid = (props.jobId || "").trim();
    if (!jid) {
      return;
    }
    setSaveErr(null);
    setSaving(true);
    try {
      const res = paused
        ? await schedulerResumeJob(jid)
        : await schedulerPauseJob(jid);
      if (!res.ok) {
        setSaveErr(res.message);
        return;
      }
      setPaused(!paused);
      props.onSaved();
    } finally {
      setSaving(false);
    }
  }

  async function onDelete() {
    if (props.mode !== "edit") {
      return;
    }
    const jid = (props.jobId || "").trim();
    if (!jid) {
      return;
    }
    const ok = window.confirm(`Delete scheduler job "${jid}"?`);
    if (!ok) {
      return;
    }
    setSaveErr(null);
    setSaving(true);
    try {
      const res = await schedulerDeleteJob(jid);
      if (!res.ok) {
        setSaveErr(res.message);
        return;
      }
      setSchedulerListHash();
      props.onDeleted();
    } finally {
      setSaving(false);
    }
  }

  const sheet = (
    <>
      <button
        type="button"
        className="scheduler-editor-backdrop"
        aria-label="Close editor"
        data-testid="scheduler-editor-backdrop"
        onClick={props.onClose}
      />
      <div
        className="scheduler-editor-panel"
        role="dialog"
        aria-modal
        aria-label={props.mode === "create" ? "New scheduler job" : "Edit scheduler job"}
        data-testid="scheduler-editor-panel"
      >
        <div className="scheduler-editor-head">
          <span>
            {props.mode === "create" ? "New job" : `Job ${jobIdField || props.jobId || ""}`}
          </span>
          <button
            type="button"
            className="sessions-close"
            aria-label="Close editor"
            data-testid="scheduler-editor-close"
            onClick={props.onClose}
          >
            ×
          </button>
        </div>

        <div className="scheduler-editor-scroll">
          {loadErr ? (
            <div className="sessions-empty" data-testid="scheduler-editor-load-err">
              {loadErr}
            </div>
          ) : null}
          {props.mode === "edit" && loading ? (
            <div className="sessions-empty">Loading…</div>
          ) : null}

          {!loadErr && (props.mode === "create" || !loading) ? (
            <div className="scheduler-editor-form">
              <label className="scheduler-field">
                <span className="scheduler-field-label">job_id</span>
                <span className="scheduler-field-help">
                  Filename - letters, digits, hyphens (example: daily-report).
                </span>
                <input
                  className={[
                    "scheduler-field-input",
                    fieldErrs.jobId ? "scheduler-field-input-err" : "",
                  ]
                    .filter(Boolean)
                    .join(" ")}
                  value={jobIdField}
                  disabled={props.mode === "edit" || saving}
                  onChange={(ev) => setJobIdField(ev.target.value)}
                  autoComplete="off"
                  spellCheck={false}
                />
                {fieldErrs.jobId ? (
                  <div className="scheduler-field-err">{fieldErrs.jobId}</div>
                ) : null}
              </label>
              <label className="scheduler-field">
                <span className="scheduler-field-label">description</span>
                <input
                  className={[
                    "scheduler-field-input",
                    fieldErrs.description ? "scheduler-field-input-err" : "",
                  ]
                    .filter(Boolean)
                    .join(" ")}
                  value={description}
                  disabled={saving}
                  onChange={(ev) => setDescription(ev.target.value)}
                />
                {fieldErrs.description ? (
                  <div className="scheduler-field-err">
                    {fieldErrs.description}
                  </div>
                ) : null}
              </label>
              <label className="scheduler-field">
                <span className="scheduler-field-label">schedule (UTC, 5 fields)</span>
                <input
                  className={[
                    "scheduler-field-input",
                    "scheduler-field-input-cron",
                    fieldErrs.schedule ? "scheduler-field-input-err" : "",
                  ]
                    .filter(Boolean)
                    .join(" ")}
                  value={schedule}
                  disabled={saving}
                  onChange={(ev) => setSchedule(ev.target.value)}
                  spellCheck={false}
                  placeholder="0 * * * *"
                />
                {fieldErrs.schedule ? (
                  <div className="scheduler-field-err">{fieldErrs.schedule}</div>
                ) : null}
              </label>
              <div
                className={
                  cronHint.ok
                    ? "scheduler-cron-hint"
                    : "scheduler-cron-hint scheduler-cron-hint-err"
                }
                data-testid="scheduler-cron-hint"
              >
                {cronHint.ok ? cronHint.text : cronHint.error}
              </div>
              <label className="scheduler-field">
                <span className="scheduler-field-label">cwd (optional)</span>
                <span className="scheduler-field-help">
                  Defaults to the agent working directory for this instance.
                </span>
                <input
                  className="scheduler-field-input"
                  value={cwd}
                  disabled={saving}
                  onChange={(ev) => setCwd(ev.target.value)}
                  placeholder={props.currentCwd || ""}
                />
              </label>
              <label className="scheduler-field">
                <span className="scheduler-field-label">mode</span>
                <select
                  className="scheduler-field-input"
                  value={modeField}
                  disabled={saving}
                  onChange={(ev) => setModeField(ev.target.value)}
                >
                  <option value="agent">agent</option>
                  <option value="plan">plan</option>
                </select>
              </label>
              <label className="scheduler-field">
                <span className="scheduler-field-label">model</span>
                {props.availableModels.length > 0 ? (
                  <select
                    className="scheduler-field-input"
                    value={model}
                    disabled={saving}
                    onChange={(ev) => setModel(ev.target.value)}
                  >
                    {props.availableModels.map((m) => (
                      <option key={m} value={m}>
                        {m}
                      </option>
                    ))}
                  </select>
                ) : (
                  <input
                    className="scheduler-field-input"
                    value={model}
                    disabled={saving}
                    onChange={(ev) => setModel(ev.target.value)}
                    spellCheck={false}
                    placeholder={props.defaultModel || ""}
                  />
                )}
              </label>
              <div className="scheduler-field scheduler-field-stack">
                <span className="scheduler-field-label">body (markdown)</span>
                <div
                  className={[
                    "scheduler-body-editor-wrap",
                    fieldErrs.body ? "scheduler-body-editor-wrap-err" : "",
                  ]
                    .filter(Boolean)
                    .join(" ")}
                >
                  <MarkdownLineEditor
                    value={body}
                    disabled={saving}
                    onChange={setBody}
                    aria-label="Job body markdown"
                    placeholder="Instruction for the scheduled run…"
                  />
                </div>
                {fieldErrs.body ? (
                  <div className="scheduler-field-err">{fieldErrs.body}</div>
                ) : null}
              </div>
              {saveErr ? (
                <div className="scheduler-save-err" data-testid="scheduler-editor-save-err">
                  {saveErr}
                </div>
              ) : null}
            </div>
          ) : null}
        </div>

        <div className="scheduler-editor-footer">
          <button
            type="button"
            className="scheduler-btn scheduler-btn-primary"
            disabled={saving || !!loadErr || (props.mode === "edit" && loading)}
            data-testid="scheduler-editor-save"
            onClick={() => void onSave()}
          >
            Save
          </button>
          {props.mode === "edit" && !loading && !loadErr ? (
            <button
              type="button"
              className="scheduler-btn"
              disabled={saving}
              data-testid="scheduler-editor-pause-toggle"
              onClick={() => void onPauseToggle()}
            >
              {paused ? "Resume" : "Pause"}
            </button>
          ) : null}
          {props.mode === "edit" ? (
            <button
              type="button"
              className="scheduler-btn scheduler-btn-danger"
              disabled={saving || loading}
              data-testid="scheduler-editor-delete"
              onClick={() => void onDelete()}
            >
              Delete
            </button>
          ) : null}
        </div>
      </div>
    </>
  );

  return createPortal(sheet, document.body);
}
