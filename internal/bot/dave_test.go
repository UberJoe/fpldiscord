package bot

import (
	"log/slog"
	"testing"
)

// recordingResponder is the seam-4 fake: it records what a handler tried to send
// instead of talking to Discord.
type recordingResponder struct {
	messages []string
	err      error
}

func (r *recordingResponder) Respond(content string) error {
	r.messages = append(r.messages, content)
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
	for _, want := range []string{"dave", "standings", "scores"} {
		if !got[want] {
			t.Errorf("commandSpecs() missing %q; have %v", want, got)
		}
	}
}
