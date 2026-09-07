// View — Bet leaderboard. The current season only, computed live: each bettor's
// four players with their cumulative season goals, a running total, a status
// marker (in / 🕓 provisionally out / 💥 bust) and the leader. The server
// pre-sorts closest to 21 from below with busts last, so this renders array
// order. Polls /api/bet at the server-provided cadence while mounted.

import { fetchBet, BetBettor, BetStatus, StartingUpError } from "../api";
import { usePoll } from "../usePoll";

const STATUS_LABEL: Record<BetStatus, string> = {
  in: "in",
  provisionallyOut: "🕓",
  bust: "💥",
};

function Bettor({
  bettor,
  seasonOver,
}: {
  bettor: BetBettor;
  seasonOver: boolean;
}) {
  const leaderMark = bettor.leader ? (seasonOver ? "🏆" : "★") : "";
  return (
    <li className={`bet-row ${bettor.status}`}>
      <div className="bet-head">
        <span className="bettor">{bettor.displayName}</span>
        <span className={`bet-status ${bettor.status}`}>
          {STATUS_LABEL[bettor.status]}
        </span>
        <span className="bet-total" title="total goals">
          {bettor.total}
        </span>
        {leaderMark && (
          <span className="bet-leader" title={seasonOver ? "winner" : "leader"}>
            {leaderMark}
          </span>
        )}
      </div>
      <ul className="bet-picks">
        {bettor.picks.map((p, i) => (
          <li key={p.elementId || i} className={p.goals === 0 ? "pick zero" : "pick"}>
            <span className="pick-name">{p.webName || `#${p.elementId}`}</span>
            <span className="pick-goals">{p.goals}</span>
          </li>
        ))}
      </ul>
    </li>
  );
}

export function Bet() {
  const { envelope, error, loading } = usePoll(fetchBet);

  if (loading && !envelope) return <p className="status">Loading the bet…</p>;
  if (error && !envelope) {
    const msg =
      error instanceof StartingUpError
        ? "The bot is still starting up…"
        : `Couldn't load the bet: ${error.message}`;
    return <p className="status">{msg}</p>;
  }
  if (!envelope) return null;

  const { season, bettors } = envelope.data;
  const seasonOver =
    envelope.meta.currentGw === 38 && envelope.meta.gwFinished;

  return (
    <div className="bet">
      {envelope.meta.stale && (
        <p className="stale-banner">
          Showing last known data — the Draft API is unreachable.
        </p>
      )}
      <div className="bet-title">
        <span className="bet-season">{season}</span>
        <span className="bet-target">closest to 21</span>
      </div>
      {bettors.length === 0 ? (
        <p className="status">No bets have been entered yet.</p>
      ) : (
        <ul className="bet-list">
          {bettors.map((b, i) => (
            <Bettor key={i} bettor={b} seasonOver={seasonOver} />
          ))}
        </ul>
      )}
    </div>
  );
}
