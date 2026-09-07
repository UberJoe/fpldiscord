// View — manager drill-down (route /manager/:id). One manager's live gameweek
// squad with auto-subs already applied: scoring XI then bench, each ordered by
// position then squad slot, with points per player, the gameweek total, and
// ▲/▼ markers for auto-subbed players. Polls /api/manager/{id} at the
// server-provided cadence while mounted. No fixtures, goalscorers or bonus
// breakdown — see the spec.

import { fetchManager, ManagerPlayer, NotFoundError, StartingUpError } from "../api";
import { usePoll } from "../usePoll";
import { navigate } from "../router";

const POS_LABEL: Record<number, string> = { 1: "GKP", 2: "DEF", 3: "MID", 4: "FWD" };

function group(players: ManagerPlayer[], inXI: boolean): ManagerPlayer[] {
  return players
    .filter((p) => p.inScoringXI === inXI)
    .sort((a, b) => a.pos - b.pos || a.squadSlot - b.squadSlot);
}

function PlayerRow({ p }: { p: ManagerPlayer }) {
  return (
    <li className="player-row">
      <span className="pos">{POS_LABEL[p.pos] ?? "?"}</span>
      <span className="pname">
        <span className="pweb">{p.webName}</span>
        <span className="pteam">{p.teamShort}</span>
        {p.autoSubbedIn && (
          <span className="sub in" title="auto-subbed on">▲</span>
        )}
        {p.autoSubbedOut && (
          <span className="sub out" title="auto-subbed off">▼</span>
        )}
      </span>
      <span className="pmin">{p.minutes}′</span>
      <span className="ppts">{p.points}</span>
    </li>
  );
}

export function Manager({ entryId }: { entryId: number }) {
  const { envelope, error, loading } = usePoll(() => fetchManager(entryId));

  const back = (
    <button className="back" onClick={() => navigate("/")}>
      ← Standings
    </button>
  );

  if (loading && !envelope) {
    return (
      <div className="manager">
        {back}
        <p className="status">Loading squad…</p>
      </div>
    );
  }
  if (error && !envelope) {
    let msg = `Couldn't load this squad: ${error.message}`;
    if (error instanceof NotFoundError) {
      msg = "That manager isn't in this league.";
    } else if (error instanceof StartingUpError) {
      msg = "The bot is still starting up…";
    }
    return (
      <div className="manager">
        {back}
        <p className="status">{msg}</p>
      </div>
    );
  }
  if (!envelope) return null;

  const m = envelope.data;
  const xi = group(m.players, true);
  const bench = group(m.players, false);

  return (
    <div className="manager">
      {back}
      {envelope.meta.stale && (
        <p className="stale-banner">
          Showing last known data — the Draft API is unreachable.
        </p>
      )}
      <div className="manager-head">
        <span className="mowner">{m.ownerName}</span>
        <span className="mgw">
          GW{m.gw} · {m.provisional ? "provisional" : "final"}
        </span>
        <span className="mtotal">{m.total}</span>
      </div>
      <ol className="player-list">
        {xi.map((p) => (
          <PlayerRow key={p.squadSlot} p={p} />
        ))}
        <li className="player-list-sep">Bench</li>
        {bench.map((p) => (
          <PlayerRow key={p.squadSlot} p={p} />
        ))}
      </ol>
    </div>
  );
}
