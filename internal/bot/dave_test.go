package bot

import (
	"log/slog"
	"testing"

	"github.com/UberJoe/fpldiscord/internal/config"
	"github.com/UberJoe/fpldiscord/internal/fpl"
	"github.com/bwmarrin/discordgo"
)

// newTestBot builds a Bot with the real handler/autocomplete wiring but no
// gateway connection — enough to assert on commandSpecs() and the dispatch
// maps.
func newTestBot() (*Bot, error) {
	log := slog.New(slog.NewTextHandler(discardWriter{}, nil))
	return New(config.Config{DiscordToken: "test-token"}, log, snapSource{}, nil)
}

// snapSource is a SnapshotSource that has not built a snapshot yet.
type snapSource struct{}

func (snapSource) Current() *fpl.Snapshot { return nil }

// recordingResponder is the seam-4 fake: it records what a handler tried to send
// instead of talking to Discord. Plain-text messages land in messages; each
// RespondEmbeds call appends its embed slice to embeds, so a test can assert on
// the number of messages and the structured fields of each.
type recordingResponder struct {
	messages []string
	embeds   [][]*discordgo.MessageEmbed
	err      error
}

func (r *recordingResponder) Respond(content string) error {
	r.messages = append(r.messages, content)
	return r.err
}

func (r *recordingResponder) RespondEmbeds(embeds []*discordgo.MessageEmbed) error {
	r.embeds = append(r.embeds, embeds)
	return r.err
}

func TestHandleDave_RepliesUnconditionally(t *testing.T) {
	r := &recordingResponder{}
	if err := handleDave(&cmdInput{resp: r}); err != nil {
		t.Fatalf("handleDave() error: %v", err)
	}
	if len(r.messages) != 1 || r.messages[0] != "fuck you Dave" {
		t.Fatalf("messages = %v, want [\"fuck you Dave\"]", r.messages)
	}
}

// TestOnReady_ShortCircuitsAfterSync proves the reconnect guard: once synced is
// set, onReady returns without touching the (here nil) session or ready payload.
func TestOnReady_ShortCircuitsAfterSync(t *testing.T) {
	b := &Bot{log: slog.New(slog.NewTextHandler(discardWriter{}, nil))}
	b.synced.Store(true)
	b.onReady(nil, nil) // must not panic — a second READY after a resume is a no-op
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

func TestCommandSpecs_RegistersCommands(t *testing.T) {
	got := map[string]bool{}
	for _, s := range commandSpecs() {
		got[s.Name] = true
	}
	for _, want := range []string{"dave", "standings", "scores", "owner", "teamlist"} {
		if !got[want] {
			t.Errorf("commandSpecs() missing %q; have %v", want, got)
		}
	}
}

// TestCommandSpecs_AutocompleteArgsAreRegistered guards the wiring the client
// depends on: an autocompleting arg must carry Autocomplete=true and the bot
// must have a resolver registered for that command.
func TestCommandSpecs_AutocompleteArgsAreRegistered(t *testing.T) {
	b, err := newTestBot()
	if err != nil {
		t.Fatalf("newTestBot: %v", err)
	}
	want := map[string]string{"owner": "player", "teamlist": "owner"}
	for _, s := range commandSpecs() {
		optName, autocompleted := want[s.Name]
		if !autocompleted {
			continue
		}
		if _, ok := b.autocomplete[s.Name]; !ok {
			t.Errorf("command %q has no autocomplete resolver registered", s.Name)
		}
		var found bool
		for _, o := range s.Options {
			if o.Name == optName {
				found = true
				if !o.Autocomplete {
					t.Errorf("%s.%s option is missing Autocomplete=true", s.Name, optName)
				}
			}
		}
		if !found {
			t.Errorf("command %q missing expected option %q", s.Name, optName)
		}
	}
}
