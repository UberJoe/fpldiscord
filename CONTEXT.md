# fpldiscord

A Discord bot and companion web view for a private Fantasy Premier League **Draft**
league: it mirrors the league's standings, live gameweek scores, waivers, and a
season-long side bet.

## Language

### Scoring

**Live total**:
A manager's score at this moment: their points accrued from completed gameweeks
plus their live score from the gameweek in progress. Updates in real time while
matches are on.
_Avoid_: current total, running total

**Frozen total**:
A manager's points accrued from completed gameweeks only, excluding the gameweek
in progress. The [[live-total]] minus the live gameweek score.
_Avoid_: last week's total, starting total, season total

**Live gameweek points**:
A manager's score in the gameweek currently in progress, with auto-substitutions
applied and provisional bonus points included.
_Avoid_: GW points, event total

**Official total** (Draft):
The cumulative season total the Draft API reports on a standings row. Includes the
current gameweek's *official* event total, which lags live scoring (no live bonus,
auto-subs only once the gameweek finishes). Not the same as the [[live-total]].
_Avoid_: Draft total
