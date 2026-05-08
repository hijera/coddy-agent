export function NavRail(props: { onNewChat: () => void; onToggleMenu: () => void; menuOpen: boolean }) {
  return (
    <aside className="rail" aria-label="Nav">
      <div className="rail-pill">
        <button type="button" className="rail-icon" title="Menu" aria-label="Menu" onClick={props.onToggleMenu}>
          <span aria-hidden="true">≡</span>
        </button>

        <div className="rail-brand" aria-label="Coddy chat">
          <div className="rail-logo" aria-hidden="true" />
          <div className="rail-brand-text">
            <div className="rail-brand-title">Coddy</div>
            <div className="rail-brand-sub">chat</div>
          </div>
        </div>

        <button type="button" className="rail-icon rail-accent" id="btn-new" title="New chat" onClick={props.onNewChat}>
          <span aria-hidden="true">＋</span>
        </button>

        <div className="rail-spacer" />

        <button type="button" className={`rail-icon ${props.menuOpen ? 'is-active' : ''}`} title="Chats" aria-label="Chats">
          <span aria-hidden="true">▦</span>
        </button>

        <button type="button" className="rail-icon" title="Profile" aria-label="Profile">
          <span aria-hidden="true">○</span>
        </button>
      </div>
    </aside>
  );
}

