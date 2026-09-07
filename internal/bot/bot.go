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
// until snapshot #1) and the responder. It is a plain value object, not a
// context.Context. Later tickets add parsed options and the store / bet
// dependencies here.
type cmdInput struct {
	snap *fpl.Snapshot
	resp Responder
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
	synced     atomic.Bool // set once the command sync has succeeded
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
	}
}

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

// onInteraction dispatches an application-command interaction to its handler.
func (b *Bot) onInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	name := i.ApplicationCommandData().Name
	h, ok := b.handlers[name]
	if !ok {
		b.log.Warn("no handler for command", "command", name)
		return
	}
	in := &cmdInput{resp: &interactionResponder{s: s, i: i}}
	if b.snap != nil {
		in.snap = b.snap.Current()
	}
	if err := h(in); err != nil {
		b.log.Error("command handler failed", "command", name, "err", err)
	}
}

// interactionResponder is the discordgo-backed Responder.
type interactionResponder struct {
	s *discordgo.Session
	i *discordgo.InteractionCreate
}

func (r *interactionResponder) Respond(content string) error {
	return r.s.InteractionRespond(r.i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: content},
	})
}
