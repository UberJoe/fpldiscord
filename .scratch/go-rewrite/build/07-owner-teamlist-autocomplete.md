# 07 — `owner` + `teamlist` commands + autocomplete infrastructure

**Spec:** [../to-spec.md](../to-spec.md)

**What to build:** `/owner <player>` names the manager who owns a player (or "free
agent"), and `/teamlist <owner>` lists a manager's squad grouped GK/DEF/MID/FWD. Both
take autocompleting arguments, and this ticket establishes the reusable autocomplete
arg-resolver used by later commands.

**Blocked by:** 02

**Status:** ready-for-agent

- [ ] `/owner <player>` returns the current owner or "free agent", read from live
      `element-status` so it is correct immediately after the GW20 redraft
- [ ] `/teamlist <owner>` returns the squad grouped by position (GK / DEF / MID / FWD)
- [ ] Player-name args autocomplete from `elements[].web_name`; owner-name args from
      `league_entries[].player_first_name`
- [ ] The autocomplete resolver is reusable, not inlined per command
- [ ] `TeamPlayers()` derived view backs both commands
- [ ] seam-4 handler + autocomplete tests against a fake snapshot
