import { type ReactElement, useCallback, useEffect, useMemo, useState } from 'react';

function safePrettyJSON(text: string): string {
  try {
    const v = JSON.parse(text);
    return JSON.stringify(v, null, 2);
  } catch {
    return text;
  }
}

function formatDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) return '';
  if (ms >= 60_000) {
    const mins = ms / 60_000;
    const fixed = mins < 10 ? mins.toFixed(1) : mins.toFixed(0);
    return `${fixed}m`;
  }
  return `${Math.round(ms)}ms`;
}

export function ToolCallMessage(props: {
  toolCallId: string;
  title?: string | undefined;
  kind?: string | undefined;
  status: string;
  argsText?: string | undefined;
  resultText?: string | undefined;
  fullResultText?: string | undefined;
  resultWasTruncated?: boolean | undefined;
  durationMs?: number;
  onFetchToolCallFull?: (toolCallId: string) => Promise<void>;
}) {
  const args = useMemo(() => (props.argsText ? safePrettyJSON(props.argsText) : ''), [props.argsText]);
  const preview = useMemo(() => (props.resultText ? props.resultText : ''), [props.resultText]);
  const full = props.fullResultText || '';
  const name = (props.title || props.kind || 'tool').trim();
  const status = (props.status || '').toLowerCase();
  const showSpinner = status === 'pending' || status === 'in_progress';

  const [showExpanded, setShowExpanded] = useState(false);
  const [loadingFull, setLoadingFull] = useState(false);

  useEffect(() => {
    setShowExpanded(false);
    setLoadingFull(false);
  }, [props.toolCallId]);

  const canExpand =
    props.resultWasTruncated === true && (status === 'completed' || status === 'failed' || status === 'cancelled');
  const fetchFull = props.onFetchToolCallFull;

  const onLoadMore = useCallback(async () => {
    if (!fetchFull) return;
    if (full) {
      setShowExpanded(true);
      return;
    }
    setLoadingFull(true);
    try {
      await fetchFull(props.toolCallId);
      setShowExpanded(true);
    } finally {
      setLoadingFull(false);
    }
  }, [fetchFull, full, props.toolCallId]);

  const onHide = useCallback(() => setShowExpanded(false), []);

  const resultBody = showExpanded && full ? full : preview;
  const useTallViewport =
    props.resultWasTruncated === true || (showExpanded && full.trim() !== '');

  const showToggleRow = canExpand && !!fetchFull && !!(preview || full);
  let toggleLink: ReactElement | null = null;
  if (showToggleRow) {
    if (showExpanded && full) {
      toggleLink = (
        <button
          type="button"
          className="tool-result-text-link"
          data-testid="tool-result-hide-link"
          onClick={(e) => {
            e.preventDefault();
            onHide();
          }}
        >
          Hide
        </button>
      );
    } else {
      toggleLink = (
        <button
          type="button"
          className="tool-result-text-link"
          data-testid="tool-result-more-link"
          disabled={loadingFull}
          onClick={(e) => {
            e.preventDefault();
            void onLoadMore();
          }}
        >
          {loadingFull ? 'Loading...' : 'Load more results'}
        </button>
      );
    }
  }

  const viewportMode = showExpanded && full ? 'scroll' : 'clip';

  const dur =
    typeof props.durationMs === 'number' && Number.isFinite(props.durationMs) && props.durationMs >= 0
      ? formatDuration(props.durationMs)
      : '';

  return (
    <div className="msg msg-tools msg-compact" data-kind={props.kind || ''} data-status={props.status}>
      <details className="tool-details" data-testid={`tool-details-${props.toolCallId}`}>
        <summary className="tool-summary" aria-label="Tool summary" title="Click to expand">
          <span className="tool-left">
            <span className={`tool-dot tool-dot-${status || 'unknown'}`} aria-hidden="true" />
            {showSpinner ? <span className="tool-spinner" aria-hidden="true" /> : null}
            <span className="tool-name">{name}</span>
          </span>
          {dur ? (
            <span className="tool-dur" aria-hidden="true">
              {dur}
            </span>
          ) : null}
        </summary>
        {args ? (
          <pre className="tool-block" aria-label="Tool arguments">
            {args}
          </pre>
        ) : null}
        {resultBody ? (
          <div
            className={[
              'tool-block tool-result tool-result-raw',
              useTallViewport && `tool-result-viewport tool-result-viewport--tall tool-result-viewport--${viewportMode}`,
            ]
              .filter(Boolean)
              .join(' ')}
            aria-label="Tool result"
          >
            <pre className="tool-result-pre">{resultBody}</pre>
          </div>
        ) : null}
        {toggleLink ? <div className="tool-result-toggle-row">{toggleLink}</div> : null}
      </details>
    </div>
  );
}
