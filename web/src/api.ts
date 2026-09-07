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
  lastRank: number; // position in last week's final standings; 0 = no previous position
  liveRank: number; // joint rank — ties share it
  arrow: number; // places moved since last week (lastRank − liveRank); 0 when lastRank is 0
  totalPoints: number;
  liveGwPoints: number;
  livePoints: number;
}

export interface StandingsData {
  rows: StandingsRow[];
}

export interface ManagerPlayer {
  elementId: number;
  webName: string;
  teamShort: string;
  pos: number; // 1=GK 2=DEF 3=MID 4=FWD
  squadSlot: number; // 1..15
  points: number;
  minutes: number;
  inScoringXI: boolean;
  autoSubbedIn: boolean;
  autoSubbedOut: boolean;
}

export interface ManagerData {
  entryId: number;
  ownerName: string;
  gw: number;
  provisional: boolean;
  total: number;
  players: ManagerPlayer[];
}

export interface WaiversRow {
  ownerName: string;
  entryId: number;
  in: string;
  out: string;
  type: "waiver" | "freeAgent";
  status: "accepted" | "failed";
  priority: number;
  index: number;
}

export interface WaiversData {
  gw: number;
  rows: WaiversRow[];
}

export interface BetPick {
  elementId: number;
  webName: string;
  goals: number;
}

export type BetStatus = "in" | "provisionallyOut" | "bust";

export interface BetBettor {
  displayName: string;
  picks: BetPick[]; // always 4, in slot order
  total: number;
  status: BetStatus;
  leader: boolean;
}

export interface BetData {
  season: string;
  bettors: BetBettor[];
}

/** Raised for a 404 from /api/manager/{id} — the id isn't in this league. */
export class NotFoundError extends Error {
  constructor(message = "not found") {
    super(message);
    this.name = "NotFoundError";
  }
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
  if (res.status === 404) {
    throw new NotFoundError();
  }
  if (!res.ok) {
    throw new Error(`${path}: HTTP ${res.status}`);
  }
  return (await res.json()) as Envelope<T>;
}

export function fetchStandings(): Promise<Envelope<StandingsData>> {
  return getEnvelope<StandingsData>("/api/standings");
}

export function fetchManager(entryId: number): Promise<Envelope<ManagerData>> {
  return getEnvelope<ManagerData>(`/api/manager/${entryId}`);
}

/**
 * Fetch one processed waiver round. Omit gw for the latest; an out-of-range gw
 * is clamped server-side and the resolved value comes back in data.gw.
 */
export function fetchWaivers(gw?: number): Promise<Envelope<WaiversData>> {
  const q = gw === undefined ? "" : `?gw=${gw}`;
  return getEnvelope<WaiversData>(`/api/waivers${q}`);
}

/** Fetch the current season's live bet leaderboard (server pre-sorts it). */
export function fetchBet(): Promise<Envelope<BetData>> {
  return getEnvelope<BetData>("/api/bet");
}
