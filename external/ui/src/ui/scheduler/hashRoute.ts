/** Hash routes: `#/s/<sessionId>`, `#/scheduler`, `#/scheduler/jobs/<job_id>`. One branch active in the URL at a time. */

export type ParsedAppHash =
  | { branch: "none" }
  | { branch: "session"; sessionId: string }
  | { branch: "scheduler"; jobId: string | null };

export function normalizeHashPath(): string {
  return window.location.hash.replace(/^#\/?/, "").trim();
}

export function parseAppHash(): ParsedAppHash {
  const h = normalizeHashPath();
  if (!h) {
    return { branch: "none" };
  }
  const schedJob = /^scheduler\/jobs\/(.+)$/.exec(h);
  if (schedJob && schedJob[1]) {
    return {
      branch: "scheduler",
      jobId: decodeURIComponent(schedJob[1]),
    };
  }
  if (h === "scheduler") {
    return { branch: "scheduler", jobId: null };
  }
  const sess = /^s\/([^/]+)$/.exec(h);
  if (sess && sess[1]) {
    return { branch: "session", sessionId: decodeURIComponent(sess[1]) };
  }
  return { branch: "none" };
}

export function setSessionHashInLocation(id: string): void {
  if (!id) {
    if (window.location.hash) {
      history.replaceState(
        null,
        "",
        `${window.location.pathname}${window.location.search}`,
      );
    }
    return;
  }
  const next = `#/s/${encodeURIComponent(id)}`;
  if (window.location.hash !== next) {
    history.replaceState(
      null,
      "",
      `${window.location.pathname}${window.location.search}${next}`,
    );
  }
}

export function setSchedulerListHash(): void {
  const next = "#/scheduler";
  if (window.location.hash !== next) {
    history.replaceState(
      null,
      "",
      `${window.location.pathname}${window.location.search}${next}`,
    );
  }
}

export function setSchedulerJobHash(jobId: string): void {
  const next = `#/scheduler/jobs/${encodeURIComponent(jobId)}`;
  if (window.location.hash !== next) {
    history.replaceState(
      null,
      "",
      `${window.location.pathname}${window.location.search}${next}`,
    );
  }
}
