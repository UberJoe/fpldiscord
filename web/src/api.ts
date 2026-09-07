// Typed client for the Go /api/* surface. Every 200 carries the shared
// {meta, data} envelope; meta is identical across endpoints and drives the poll
// cadence. See internal/web/api.go.

export interface Meta {
  matchLive: boolean;
  pollAfterMs: number;
  stale: boolean;
  builtAt: string;
  leagueName: string;
  leagueMode: "classic" | "h2h";
  currentGw: number;
  gwFinished: boolean;
  processedGws: number[];
}

export interface Envelope<T> {
  meta: Meta;
  data: T;
}

export interface StandingsRow {
  entryId: number;
  ownerName: string;
  entryName: string;
  officialRank: number;
  liveRank: number;
  arrow: number;
  totalPoints: number;
  liveGwPoints: number;
  livePoints: number;
}

export interface StandingsData {
  rows: StandingsRow[];
}

/** Raised for the pre-first-snapshot 503 so callers can show "starting up". */
export class StartingUpError extends Error {
  retryAfterMs: number;
  constructor(retryAfterMs: number) {
    super("starting up");
    this.name = "StartingUpError";
    this.retryAfterMs = retryAfterMs;
  }
}

async function getEnvelope<T>(path: string): Promise<Envelope<T>> {
  const res = await fetch(path, { headers: { Accept: "application/json" } });
  if (res.status === 503) {
    const body = await res.json().catch(() => ({}));
    const retry =
      typeof body.retryAfterMs === "number" ? body.retryAfterMs : 3000;
    throw new StartingUpError(retry);
  }
  if (!res.ok) {
    throw new Error(`${path}: HTTP ${res.status}`);
  }
  return (await res.json()) as Envelope<T>;
}

export function fetchStandings(): Promise<Envelope<StandingsData>> {
  return getEnvelope<StandingsData>("/api/standings");
}
