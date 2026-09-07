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
	if err := handleDave(r); err != nil {
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

func TestCommandSpecs_ContainsDaveOnly(t *testing.T) {
	specs := commandSpecs()
	if len(specs) != 1 {
		t.Fatalf("commandSpecs() len = %d, want 1", len(specs))
	}
	if specs[0].Name != "dave" {
		t.Errorf("command name = %q, want dave", specs[0].Name)
	}
}
