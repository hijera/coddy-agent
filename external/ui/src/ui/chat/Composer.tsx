export function Composer(props: {
  value: string;
  isEmpty: boolean;
  onChange: (v: string) => void;
  onSend: (text: string) => void;
}) {
  const sendDisabled = props.value.trim() === '';

  return (
    <footer className="composer-wrap">
      <label className="sr-only" htmlFor="composer">
        Message
      </label>
      <div className="composer-card">
        <textarea
          id="composer"
          rows={props.isEmpty ? 5 : 2}
          placeholder={props.isEmpty ? 'Ask anything...' : 'Message Coddy'}
          autoComplete="off"
          value={props.value}
          onChange={(ev) => props.onChange(ev.target.value)}
          onKeyDown={(ev) => {
            if (ev.key === 'Enter' && !ev.shiftKey) {
              ev.preventDefault();
              const txt = props.value.trim();
              if (!txt) {
                return;
              }
              props.onSend(txt);
            }
          }}
        />

        <div className="composer-bar">
          <div className="composer-tabs" aria-label="Composer options">
            <button type="button" className="composer-tab">
              Model
            </button>
            <button type="button" className="composer-tab">
              Tools
            </button>
          </div>

          <div className="composer-bar-actions">
            <button type="button" className="composer-icon" aria-label="Edit">
              <span aria-hidden="true">✎</span>
            </button>
            <button
              type="button"
              className="composer-icon composer-send"
              id="btn-send"
              aria-label="Send"
              disabled={sendDisabled}
              onClick={() => {
                const txt = props.value.trim();
                if (!txt) {
                  return;
                }
                props.onSend(txt);
              }}
            >
              <span aria-hidden="true">➤</span>
            </button>
          </div>
        </div>
      </div>
    </footer>
  );
}

