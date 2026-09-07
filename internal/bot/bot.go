// Package bot runs the Discord side of fpldiscord: a discordgo session, a
// hand-rolled command handler map, and (from ticket 12) the daily
// waiver-reminder goroutine. Ticket 01 wires the session, registers the command
// set once on READY with ApplicationCommandBulkOverwrite, and implements /dave.
package bot

import (
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/UberJoe/fpldiscord/internal/config"
	"github.com/UberJoe/fpldiscord/internal/fpl"
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

// cmdInput is what a command handler is given: the current fpl snapshot (nil
// until snapshot #1), the interaction's command options, and the responder. It
// is a plain value object, not a context.Context. Later tickets add the store /
// bet dependencies here.
type cmdInput struct {
	snap *fpl.Snapshot
	opts cmdOptions
	resp Responder
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
	session    *discordgo.Session
	log        *slog.Logger
	devGuildID string
	snap       SnapshotSource
	handlers   map[string]handlerFunc
	// autocomplete resolves the focused option of an autocomplete interaction,
	// keyed by command name. Commands without an autocompleting arg are absent.
	autocomplete map[string]acHandlerFunc
	synced       atomic.Bool // set once the command sync has succeeded
}

// New builds a Bot from config. The gateway is not opened until Open is called.
func New(cfg config.Config, log *slog.Logger, snap SnapshotSource) (*Bot, error) {
	session, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		return nil, fmt.Errorf("discordgo.New: %w", err)
	}
	// Keep RAM in the tens of MB on the 256 MB box: no member/presence cache,
	// only the intents the commands actually need.
	session.StateEnabled = false
	session.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages

	b := &Bot{
		session:    session,
		log:        log,
		devGuildID: cfg.DevGuildID,
		snap:       snap,
		handlers: map[string]handlerFunc{
			"dave":      handleDave,
			"standings": handleStandings,
			"scores":    handleScores,
			"owner":     handleOwner,
			"teamlist":  handleTeamlist,
			"waivers":   handleWaivers,
			"overview":  handleOverview,
		},
		autocomplete: map[string]acHandlerFunc{
			"owner":    autocompletePlayer,
			"teamlist": autocompleteOwner,
		},
	}

	session.AddHandler(b.onInteraction)
	session.AddHandler(b.onReady)
	return b, nil
}

// Open connects the gateway. discordgo services it on background goroutines, so
// this composes with the HTTP server in the same process.
func (b *Bot) Open() error { return b.session.Open() }

// Close disconnects the gateway.
func (b *Bot) Close() error { return b.session.Close() }

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
	}
}

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
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		b.onCommand(s, i)
	case discordgo.InteractionApplicationCommandAutocomplete:
		b.onAutocomplete(s, i)
	}
}

// onCommand dispatches a slash-command invocation to its handler.
func (b *Bot) onCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	h, ok := b.handlers[data.Name]
	if !ok {
		b.log.Warn("no handler for command", "command", data.Name)
		return
	}
	opts := make(cmdOptions, len(data.Options))
	for _, o := range data.Options {
		opts[o.Name] = o.Value
	}
	in := &cmdInput{opts: opts, resp: &interactionResponder{s: s, i: i}}
	if b.snap != nil {
		in.snap = b.snap.Current()
	}
	if err := h(in); err != nil {
		b.log.Error("command handler failed", "command", data.Name, "err", err)
	}
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
// currently editing in an autocomplete interaction.
func focusedOption(opts []*discordgo.ApplicationCommandInteractionDataOption) (name, partial string) {
	for _, o := range opts {
		if o.Focused {
			s, _ := o.Value.(string)
			return o.Name, s
		}
	}
	return "", ""
}

// interactionResponder is the discordgo-backed Responder. A handler may call
// Respond more than once (e.g. /waivers splitting a long round across
// messages): the first call is the interaction response, every later one is a
// follow-up message on the same interaction.
type interactionResponder struct {
	s        *discordgo.Session
	i        *discordgo.InteractionCreate
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
	return r.s.InteractionRespond(r.i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: content},
	})
}
