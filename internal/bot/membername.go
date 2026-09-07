package bot

import "time"

// memberNameTTL is how long a resolved id->name entry is trusted before the
// next lookup refreshes it. A display-name change picks up within an hour.
const memberNameTTL = time.Hour

type memberNameEntry struct {
	name string
	at   time.Time
}

// rememberGuild records the guild an interaction arrived from, so MemberName has
// a guild to do its REST lookups against. This bot serves a single guild; the
// first interaction fixes it.
func (b *Bot) rememberGuild(guildID string) {
	if guildID == "" {
		return
	}
	b.nameMu.Lock()
	if b.guildID == "" {
		b.guildID = guildID
	}
	b.nameMu.Unlock()
}

// MemberName resolves a Discord user id to that member's guild display name,
// caching each result for ~memberNameTTL. On a cache miss it does one lazy REST
// GuildMember lookup (StateEnabled is off, so there is no gateway member cache);
// a failed lookup returns ok=false and is not cached. Before the first
// interaction — no guild known yet — every call misses.
func (b *Bot) MemberName(discordUserID string) (string, bool) {
	b.nameMu.Lock()
	entry, cached := b.nameCache[discordUserID]
	guildID := b.guildID
	b.nameMu.Unlock()

	if cached && time.Since(entry.at) < memberNameTTL {
		return entry.name, entry.name != ""
	}
	if guildID == "" || b.session == nil {
		return "", false
	}

	m, err := b.session.GuildMember(guildID, discordUserID)
	if err != nil || m == nil {
		return "", false
	}
	name := m.DisplayName()

	b.nameMu.Lock()
	b.nameCache[discordUserID] = memberNameEntry{name: name, at: time.Now()}
	b.nameMu.Unlock()
	return name, name != ""
}
