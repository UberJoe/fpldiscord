// Package bot runs the Discord side of fpldiscord: a discordgo session, a
// hand-rolled command handler map, and (from ticket 12) the daily
// waiver-reminder goroutine. Ticket 01 wires the session, registers the command
// set once on READY with ApplicationCommandBulkOverwrite, and implements /dave.
package bot

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/UberJoe/fpldiscord/internal/config"
	"github.com/UberJoe/fpldiscord/internal/fpl"
	"github.com/UberJoe/fpldiscord/internal/store"
	"github.com/bwmarrin/discordgo"
)

// Responder is the narrow output side a command handler needs: send one message
// back to the caller. The real implementation talks to discordgo; tests pass a
// fake that records what was sent.
type Responder interface {
	Respond(content string) error
}

// SnapshotSource is the read side of fpl.Store the bot needs: the current
// snapshot, or nil before the first successful build.
type SnapshotSource interface {
	Current() *fpl.Snapshot
}

// BetStore is the slice of store.Store the bet command needs: read the current
// picks and archived seasons, and write picks / archive rows. A consumer-side
// interface so the /bet handlers can be exercised with a fake at seam 4.
type BetStore interface {
	CurrentPicks(season string) ([]store.BettorPicks, error)
	SetPicks(season, discordUserID string, elements [4]int) error
	ArchivedSeasons() ([]string, error)
	Archive(season string) ([]store.ArchivedBettor, error)
	AddArchive(season, bettorName string, picks [4]store.ArchivePick) error
}

// MemberNamer resolves a Discord user id to a guild display name. *Bot
// implements it (a mutex-guarded id->name cache over lazy REST GuildMember
// lookups); it is handed to the /bet leaderboard renderer and, via app, to the
// web package.
type MemberNamer interface {
	MemberName(discordUserID string) (string, bool)
}

// cmdInput is what a command handler is given: the current fpl snapshot (nil
// until snapshot #1), the interaction's command options, and the responder. It
// is a plain value object, not a context.Context.
//
// The remaining fields are only populated for grouped / gated commands (/bet):
// sub is the invoked subcommand name; caller is the invoker's Discord user id;
// betStore, season and isAdmin wire the bet game; namer resolves bettor ids to
// display names.
type cmdInput struct {
	snap *fpl.Snapshot
	opts cmdOptions
	resp Responder

	sub      string
	caller   string
	betStore BetStore
	season   string
	isAdmin  func(discordUserID string) bool
	namer    MemberNamer
}

// cmdOptions holds an interaction's command options keyed by name. Values are
// the raw discordgo option values.
type cmdOptions map[string]any

// Int returns a named integer option. discordgo delivers integer options as
// float64 (JSON numbers); a missing or non-numeric option returns ok=false.
func (o cmdOptions) Int(name string) (int, bool) {
	switch n := o[name].(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

// String returns a named string option; a missing or non-string option returns
// ok=false.
func (o cmdOptions) String(name string) (string, bool) {
	s, ok := o[name].(string)
	return s, ok
}

// handlerFunc is one command handler, dispatched by name from the hand-rolled
// map.
type handlerFunc func(in *cmdInput) error

// Bot owns the discordgo session and the command dispatch table.
type Bot struct {
	session               *discordgo.Session
	log                   *slog.Logger
	devGuildID            string
	adminIDs              []string
	season                string
	notificationChannelID string
	snap                  SnapshotSource
	betStore              BetStore
	handlers              map[string]handlerFunc
	// autocomplete resolves the focused option of an autocomplete interaction,
	// keyed by command name. Commands without an autocompleting arg are absent.
	autocomplete map[string]acHandlerFunc
	synced       atomic.Bool // set once the command sync has succeeded

	// Member display-name cache for the /bet leaderboard (and, via app, the web
	// /api/bet endpoint). guildID is captured from the first interaction seen.
	nameMu    sync.Mutex
	nameCache map[string]memberNameEntry
	guildID   string
}

// New builds a Bot from config. The gateway is not opened until Open is called.
func New(cfg config.Config, log *slog.Logger, snap SnapshotSource, betStore BetStore) (*Bot, error) {
	session, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		return nil, fmt.Errorf("discordgo.New: %w", err)
	}
	// Keep RAM in the tens of MB on the 256 MB box: no member/presence cache,
	// only the intents the commands actually need.
	session.StateEnabled = false
	session.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages

	b := &Bot{
		session:               session,
		log:                   log,
		devGuildID:            cfg.DevGuildID,
		adminIDs:              cfg.AdminIDs,
		season:                cfg.Season,
		notificationChannelID: cfg.NotificationChannelID,
		snap:                  snap,
		betStore:              betStore,
		nameCache:             map[string]memberNameEntry{},
		handlers: map[string]handlerFunc{
			"dave":      handleDave,
			"standings": handleStandings,
			"scores":    handleScores,
			"owner":     handleOwner,
			"teamlist":  handleTeamlist,
			"waivers":   handleWaivers,
			"overview":  handleOverview,
			"bet":       handleBet,
		},
		autocomplete: map[string]acHandlerFunc{
			"owner":    autocompletePlayer,
			"teamlist": autocompleteOwner,
			"bet":      autocompletePlayer,
		},
	}

	session.AddHandler(b.onInteraction)
	session.AddHandler(b.onReady)
	return b, nil
}

// isAdmin reports whether a Discord user id is on the configured admin
// allowlist (ADMIN_IDS). It gates /bet set and /bet archive.
func (b *Bot) isAdmin(discordUserID string) bool {
	for _, id := range b.adminIDs {
		if id == discordUserID {
			return true
		}
	}
	return false
}

// Open connects the gateway. discordgo services it on background goroutines, so
// this composes with the HTTP server in the same process.
func (b *Bot) Open() error { return b.session.Open() }

// Close disconnects the gateway.
func (b *Bot) Close() error { return b.session.Close() }

// RunReminder drives the daily waiver-reminder task until ctx is cancelled. app
// starts it in the run phase once the gateway is open and cancels it on
// shutdown. It posts to NOTIFICATION_CHANNEL_ID.
func (b *Bot) RunReminder(ctx context.Context) {
	newReminder(b.snap, b.session, b.notificationChannelID, b.log).Run(ctx)
}

// commandSpecs is the full desired command set sent to Discord on READY. Ticket
// 01 ships only /dave; later tickets append their commands here.
func commandSpecs() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
		{
			Name:        "dave",
			Description: "Responds with a message for whenever Dave pipes up",
		},
		{
			Name:        "standings",
			Description: "Show the classic total-points league table",
		},
		{
			Name:        "scores",
			Description: "Show every manager's live gameweek points (auto-subs applied)",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "gw",
					Description: "Gameweek to show (defaults to the current one)",
					MinValue:    &scoresGWMin,
					MaxValue:    38,
					Required:    false,
				},
			},
		},
		{
			Name:        "waivers",
			Description: "Show the results of a processed waiver round",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "gw",
					Description: "Gameweek to show (defaults to the latest processed round)",
					MinValue:    &waiversGWMin,
					MaxValue:    38,
					Required:    false,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "result",
					Description: "Which claims to show",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "accepted", Value: "accepted"},
						{Name: "failed", Value: "failed"},
						{Name: "all", Value: "all"},
					},
				},
			},
		},
		{
			Name:        "owner",
			Description: "Show which manager owns a player",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "player",
					Description:  "Player name",
					Required:     true,
					Autocomplete: true,
				},
			},
		},
		{
			Name:        "teamlist",
			Description: "List a manager's squad grouped by position",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "owner",
					Description:  "Manager's first name",
					Required:     true,
					Autocomplete: true,
				},
			},
		},
		{
			Name:        "overview",
			Description: "Show this gameweek's fixtures and goalscorers",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "mode",
					Description: "Which fixtures to show (defaults to the whole gameweek)",
					Required:    false,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "Today's matches", Value: "today"},
						{Name: "Gameweek's matches", Value: "gameweek"},
						{Name: "Live matches", Value: "live"},
					},
				},
			},
		},
		{
			Name:        "bet",
			Description: "The season-long closest-to-21 goals bet",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "show",
					Description: "Show the bet leaderboard (live, or a past season)",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "season",
							Description: "Past season to show, e.g. 2025/26 (omit for the current one)",
							Required:    false,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "set",
					Description: "Admin: set or replace a bettor's four current-season picks",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionUser,
							Name:        "bettor",
							Description: "The bettor whose picks these are",
							Required:    true,
						},
						betPlayerOption("p1", "First pick"),
						betPlayerOption("p2", "Second pick"),
						betPlayerOption("p3", "Third pick"),
						betPlayerOption("p4", "Fourth pick"),
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "archive",
					Description: "Admin: record a completed past season",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "season",
							Description: "Season being archived, e.g. 2025/26",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "bettor_name",
							Description: "Bettor's name (free text — they may have left the server)",
							Required:    true,
						},
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "entries",
							Description: "Four Name:goals entries, comma-separated — e.g. Salah:19, Isak:23, Haaland:27, Palmer:15",
							Required:    true,
						},
					},
				},
			},
		},
	}
}

// betPlayerOption builds one of the /bet set p1..p4 autocompleting player args.
func betPlayerOption(name, desc string) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:         discordgo.ApplicationCommandOptionString,
		Name:         name,
		Description:  desc,
		Required:     true,
		Autocomplete: true,
	}
}

// deferredCommands names the commands whose reply is ACK'd with a deferred
// response first (a "thinking…" placeholder), so slower rendering — /bet's
// member-name lookups — can't miss Discord's 3-second initial-response window.
var deferredCommands = map[string]bool{"bet": true}

// scoresGWMin is the /scores gw option minimum; discordgo wants a *float64.
var scoresGWMin float64 = 1

// waiversGWMin is the /waivers gw option minimum; discordgo wants a *float64.
var waiversGWMin float64 = 1

// onReady runs one ApplicationCommandBulkOverwrite — Discord diffs the set
// server-side. Guild-scoped (instant) when DEV_GUILD_ID is set, else global
// (propagation can take ~1 h). discordgo re-emits Ready on every gateway
// resume; the `synced` flag keeps this to a single successful global command
// write per process while still retrying if the first attempt errors.
func (b *Bot) onReady(s *discordgo.Session, r *discordgo.Ready) {
	if b.synced.Load() {
		return
	}
	scope := "global"
	if b.devGuildID != "" {
		scope = "guild " + b.devGuildID
	}
	if _, err := s.ApplicationCommandBulkOverwrite(r.User.ID, b.devGuildID, commandSpecs()); err != nil {
		b.log.Error("command sync failed", "scope", scope, "err", err)
		return
	}
	b.synced.Store(true)
	b.log.Info("commands synced", "count", len(commandSpecs()), "scope", scope)
}

// onInteraction routes an interaction: a slash-command invocation to its
// handler, an autocomplete request to its resolver. Everything else is ignored.
func (b *Bot) onInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	b.rememberGuild(i.GuildID)
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		b.onCommand(s, i)
	case discordgo.InteractionApplicationCommandAutocomplete:
		b.onAutocomplete(s, i)
	}
}

// onCommand dispatches a slash-command invocation to its handler. A grouped
// command (one whose sole top-level option is a subcommand — /bet) is flattened
// here: the subcommand name goes to cmdInput.sub and its args become opts.
func (b *Bot) onCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	h, ok := b.handlers[data.Name]
	if !ok {
		b.log.Warn("no handler for command", "command", data.Name)
		return
	}

	sub := ""
	srcOpts := data.Options
	if len(srcOpts) == 1 && srcOpts[0].Type == discordgo.ApplicationCommandOptionSubCommand {
		sub = srcOpts[0].Name
		srcOpts = srcOpts[0].Options
	}
	opts := make(cmdOptions, len(srcOpts))
	for _, o := range srcOpts {
		opts[o.Name] = o.Value
	}

	// /bet renders bettor names, which can mean lazy REST member lookups; ACK
	// with a deferred response so those cannot push the reply past Discord's
	// 3-second initial-response window. The first Respond then edits the
	// placeholder.
	resp := &interactionResponder{s: s, i: i}
	if deferredCommands[data.Name] {
		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		}); err != nil {
			b.log.Error("deferred ack failed", "command", data.Name, "err", err)
		} else {
			resp.deferred = true
		}
	}

	in := &cmdInput{
		opts:     opts,
		resp:     resp,
		sub:      sub,
		caller:   interactionUserID(i),
		betStore: b.betStore,
		season:   b.season,
		isAdmin:  b.isAdmin,
		namer:    b,
	}
	if b.snap != nil {
		in.snap = b.snap.Current()
	}
	if err := h(in); err != nil {
		b.log.Error("command handler failed", "command", data.Name, "err", err)
	}
}

// interactionUserID returns the invoking user's Discord id, from the guild
// member (guild commands) or the top-level user (DM fallback).
func interactionUserID(i *discordgo.InteractionCreate) string {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	}
	if i.User != nil {
		return i.User.ID
	}
	return ""
}

// onAutocomplete resolves the focused option of an autocomplete request and
// replies with the matching choices. A request that arrives before snapshot #1
// gets an empty (but valid) choice list rather than an error in the client. A
// command with no registered resolver is a wiring bug (an Autocomplete=true
// option with nothing to serve it) — it is logged and left unanswered, the
// same shape as onCommand's unknown-command path.
func (b *Bot) onAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	h, ok := b.autocomplete[data.Name]
	if !ok {
		b.log.Warn("no autocomplete resolver for command", "command", data.Name)
		return
	}

	var choices []*discordgo.ApplicationCommandOptionChoice
	if b.snap != nil {
		if snap := b.snap.Current(); snap != nil {
			focused, partial := focusedOption(data.Options)
			choices = h(snap, focused, partial)
		}
	}

	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	}); err != nil {
		b.log.Error("autocomplete respond failed", "command", data.Name, "err", err)
	}
}

// focusedOption returns the name and partial value of the option the user is
// currently editing in an autocomplete interaction. It descends into a
// subcommand's option list (/bet set p1..p4) so grouped commands resolve too.
func focusedOption(opts []*discordgo.ApplicationCommandInteractionDataOption) (name, partial string) {
	for _, o := range opts {
		if o.Focused {
			s, _ := o.Value.(string)
			return o.Name, s
		}
		if len(o.Options) > 0 {
			if n, p := focusedOption(o.Options); n != "" {
				return n, p
			}
		}
	}
	return "", ""
}

// interactionResponder is the discordgo-backed Responder. A handler may call
// Respond more than once (e.g. /waivers splitting a long round across
// messages): the first call is the interaction response, every later one is a
// follow-up message on the same interaction. When deferred is set the caller
// has already sent a deferred ACK, so the first Respond edits that placeholder
// instead of opening a fresh response.
type interactionResponder struct {
	s        *discordgo.Session
	i        *discordgo.InteractionCreate
	deferred bool
	answered bool
}

func (r *interactionResponder) Respond(content string) error {
	if r.answered {
		_, err := r.s.FollowupMessageCreate(r.i.Interaction, false, &discordgo.WebhookParams{
			Content: content,
		})
		return err
	}
	r.answered = true
	if r.deferred {
		_, err := r.s.InteractionResponseEdit(r.i.Interaction, &discordgo.WebhookEdit{
			Content: &content,
		})
		return err
	}
	return r.s.InteractionRespond(r.i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: content},
	})
}
