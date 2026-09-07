# 07 — `owner` + `teamlist` commands + autocomplete infrastructure

**Spec:** [../to-spec.md](../to-spec.md)

**What to build:** `/owner <player>` names the manager who owns a player (or "free
agent"), and `/teamlist <owner>` lists a manager's squad grouped GK/DEF/MID/FWD. Both
take autocompleting arguments, and this ticket establishes the reusable autocomplete
arg-resolver used by later commands.

**Blocked by:** 02

**Status:** done

- [x] `/owner <player>` returns the current owner or "free agent", read from live
      `element-status` so it is correct immediately after the GW20 redraft
- [x] `/teamlist <owner>` returns the squad grouped by position (GK / DEF / MID / FWD)
- [x] Player-name args autocomplete from `elements[].web_name`; owner-name args from
      `league_entries[].player_first_name`
- [x] The autocomplete resolver is reusable, not inlined per command
- [x] `TeamPlayers()` derived view backs both commands
- [x] seam-4 handler + autocomplete tests against a fake snapshot

## Notes

- `fpl.TeamPlayers()` is the new derived view: one `TeamPlayer` row per
  bootstrap element in bootstrap-static order, each joined to its owner via
  `element-status.owner` (an `EntryID`) → `league_entries`. A missing status
  row, a nil owner, or an owner id absent from the league all read as a free
  agent (zero `OwnerEntryID`, empty `OwnerName` / `OwnerTeam`). Built from the
  exported `Snapshot` fields so the bot seam builds a literal and calls it;
  slices ranged by index-into-pointer, matching `ManagerSquad`.
- `fpl.StripAccents` is now exported — the shared name-fold primitive. The bot
  resolver folds a user's partial through `strings.ToLower(fpl.StripAccents(…))`
  and compares against the snapshot's pre-stripped `PlayerNames` / `OwnerNames`.
- `internal/bot/autocomplete.go` holds the reusable resolver:
  `resolveAutocomplete([]acCandidate, partial)` → up to 25
  `*discordgo.ApplicationCommandOptionChoice`, prefix matches ranked above
  mid-string, blank partial returns the first 25. `playerCandidates` /
  `ownerCandidates` adapt the snapshot slices (owner names de-duped by first
  name). `acHandlerFunc` takes `(snap, focused, partial)` so `/bet set`'s
  multi-slot args can reuse it later.
- The bot dispatcher gained an autocomplete path: `onInteraction` now switches
  on interaction type, `onAutocomplete` finds the focused option and replies
  with `InteractionApplicationCommandAutocompleteResult`. A pre-snapshot
  autocomplete gets an empty-but-valid choice list. `cmdOptions.String` added
  alongside `.Int`.
- `/teamlist` renders one line per position (`GK`/`DEF`/`MID`/`FWD` fixed
  order), comma-joined `Name (CLUB)` in element order, `—` for an empty
  position — a 15-man squad never risks message overflow. Owners are matched on
  first name; two managers sharing one collapse to a single autocomplete entry
  and `/teamlist` shows the first seen (element order), like the Python bot's
  `get_team_id` `.iloc[0]`, rather than merging both squads.
- `bot/names.go` holds the two shared helpers: `fold` (accent-strip + trim +
  lower, used by the resolver and both commands) and `playerLabel` (the
  `Name (CLUB)` display form).
