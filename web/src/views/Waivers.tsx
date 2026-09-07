// View — Waiver History. One processed gameweek at a time, chosen with a
// selector that defaults to the latest processed round. Accepted and failed
// claims sit in one table in the server's index order, so a contested player's
// winning claim and the claims it out-bid appear together in bid order. Unlike
// Standings this view does not poll — it refetches only on mount and when the
// selected gameweek changes.

import { useEffect, useState } from "react";
import {
  Envelope,
  WaiversData,
  WaiversRow,
  fetchWaivers,
  StartingUpError,
} from "../api";

function Move({ row }: { row: WaiversRow }) {
  if (!row.out) return <span className="wmove">{row.in}</span>;
  return (
    <span className="wmove">
      <span className="wout">{row.out}</span>
      <span className="warrow"> → </span>
      <span className="win">{row.in}</span>
    </span>
  );
}

function Row({ row }: { row: WaiversRow }) {
  const bid =
    row.type === "freeAgent" ? "FA" : row.priority > 0 ? `#${row.priority}` : "—";
  return (
    <li className={`waiver-row ${row.status}`}>
      <span className="wbid" title={row.type === "freeAgent" ? "free agent" : "waiver priority"}>
        {bid}
      </span>
      <span className="wmanager">{row.ownerName || "—"}</span>
      <Move row={row} />
      <span className={`wstatus ${row.status}`}>
        {row.status === "accepted" ? "won" : "out-bid"}
      </span>
    </li>
  );
}

export function Waivers() {
  const [gw, setGw] = useState<number | undefined>(undefined);
  const [envelope, setEnvelope] = useState<Envelope<WaiversData> | null>(null);
  const [error, setError] = useState<Error | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    fetchWaivers(gw)
      .then((env) => {
        if (cancelled) return;
        setEnvelope(env);
        setLoading(false);
        // Adopt the server-resolved gameweek so the selector reflects any clamp.
        if (gw === undefined) setGw(env.data.gw);
      })
      .catch((err) => {
        if (cancelled) return;
        setError(err instanceof Error ? err : new Error(String(err)));
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [gw]);

  if (loading && !envelope) return <p className="status">Loading waivers…</p>;
  if (error && !envelope) {
    const msg =
      error instanceof StartingUpError
        ? "The bot is still starting up…"
        : `Couldn't load waivers: ${error.message}`;
    return <p className="status">{msg}</p>;
  }
  if (!envelope) return null;

  const { rows, gw: resolvedGw } = envelope.data;
  // Options are the processed rounds, plus the resolved gameweek itself in the
  // rare case it falls in a gap the processed list doesn't name.
  const processed = envelope.meta.processedGws;
  const options = processed.includes(resolvedGw)
    ? processed
    : [...processed, resolvedGw].sort((a, b) => a - b);

  return (
    <div className="waivers">
      {envelope.meta.stale && (
        <p className="stale-banner">
          Showing last known data — the Draft API is unreachable.
        </p>
      )}

      <div className="waiver-head">
        <label htmlFor="waiver-gw">Gameweek</label>
        <select
          id="waiver-gw"
          value={resolvedGw}
          disabled={processed.length === 0}
          onChange={(e) => setGw(Number(e.target.value))}
        >
          {options.map((n) => (
            <option key={n} value={n}>
              GW{n}
            </option>
          ))}
        </select>
      </div>

      {processed.length === 0 ? (
        <p className="status">No waiver rounds have been processed yet.</p>
      ) : rows.length === 0 ? (
        <p className="status">No waiver claims in GW{resolvedGw}.</p>
      ) : (
        <ul className="waiver-list">
          <li className="waiver-row head">
            <span className="wbid">Bid</span>
            <span className="wmanager">Manager</span>
            <span className="wmove">Move</span>
            <span className="wstatus">Result</span>
          </li>
          {rows.map((r) => (
            <Row key={`${r.index}-${r.entryId}`} row={r} />
          ))}
        </ul>
      )}
    </div>
  );
}
