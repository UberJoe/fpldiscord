// View 1 — Standings, also the live-scores view. Every league manager in live
// order (server pre-sorts by livePoints desc); each row shows the season total,
// the gameweek points so far, and a green/red arrow for live movement within
// the gameweek. All sorting and rank maths are server-side — this renders array
// order only. Tapping a row opens the manager drill-down route.

import { fetchStandings, StandingsRow, StartingUpError } from "../api";
import { usePoll } from "../usePoll";
import { navigate } from "../router";

function Arrow({ n }: { n: number }) {
  if (n > 0) return <span className="arrow up" title={`up ${n}`}>▲ {n}</span>;
  if (n < 0) return <span className="arrow down" title={`down ${-n}`}>▼ {-n}</span>;
  return <span className="arrow flat" title="no change">–</span>;
}

function Row({ row }: { row: StandingsRow }) {
  return (
    <li
      className="standings-row"
      onClick={() => navigate(`/manager/${row.entryId}`)}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") navigate(`/manager/${row.entryId}`);
      }}
    >
      <span className="rank">{row.liveRank}</span>
      <span className="who">
        <span className="owner">{row.ownerName}</span>
        <span className="team">{row.entryName}</span>
      </span>
      <span className="gw">
        +{row.liveGwPoints}
        <Arrow n={row.arrow} />
      </span>
      <span className="total">{row.totalPoints}</span>
    </li>
  );
}

export function Standings() {
  const { envelope, error, loading } = usePoll(fetchStandings);

  if (loading && !envelope) return <p className="status">Loading standings…</p>;
  if (error && !envelope) {
    const msg =
      error instanceof StartingUpError
        ? "The bot is still starting up…"
        : `Couldn't load standings: ${error.message}`;
    return <p className="status">{msg}</p>;
  }
  if (!envelope) return null;

  const { rows } = envelope.data;

  if (envelope.meta.leagueMode !== "classic") {
    return <p className="status">Standings aren't available in head-to-head mode.</p>;
  }
  if (rows.length === 0) {
    return <p className="status">No standings yet.</p>;
  }

  return (
    <>
      {envelope.meta.stale && (
        <p className="stale-banner">Showing last known data — the Draft API is unreachable.</p>
      )}
      <ol className="standings">
        <li className="standings-row head">
          <span className="rank">#</span>
          <span className="who">Manager</span>
          <span className="gw">GW{envelope.meta.currentGw}</span>
          <span className="total">Total</span>
        </li>
        {rows.map((r) => (
          <Row key={r.entryId} row={r} />
        ))}
      </ol>
    </>
  );
}
