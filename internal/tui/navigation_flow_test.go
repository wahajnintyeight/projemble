package tui

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/vt"
	ui "github.com/metaspartan/gotui/v5"
)

// Exercise the real event conversion and app loop, including all onboarding back routes.
func TestOnboardingEscapeFlowThroughTerminalEvents(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "")
	parent := t.TempDir()
	var keys []*tcell.EventKey
	text := func(value string) {
		for _, r := range value {
			keys = append(keys, tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone))
		}
	}
	key := func(value tcell.Key) { keys = append(keys, tcell.NewEventKey(value, "", tcell.ModNone)) }
	text("n")
	text("sample")
	key(tcell.KeyEnter)
	text("Example project")
	key(tcell.KeyEnter)
	text(parent)
	key(tcell.KeyEnter)
	key(tcell.KeyEnter) // workload
	key(tcell.KeyEnter) // patterns
	key(tcell.KeyEnter) // service topology
	key(tcell.KeyEnter) // architecture
	key(tcell.KeyDown)
	key(tcell.KeyEnter) // Go stack
	key(tcell.KeyEnter) // optional capabilities
	key(tcell.KeyEnter) // generation mode
	key(tcell.KeyDown)
	key(tcell.KeyEnter) // scaffold + agent
	key(tcell.KeyEnter) // provider -> API key
	for i := 0; i < 13; i++ {
		key(tcell.KeyEsc)
	} // back through every wizard step, then exit
	runTerminalKeyFlow(t, keys)
}

func TestHomeProviderEscapeFlowThroughTerminalEvents(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	runTerminalKeyFlow(t, []*tcell.EventKey{
		tcell.NewEventKey(tcell.KeyRune, "r", tcell.ModNone),
		tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone),
		tcell.NewEventKey(tcell.KeyEsc, "", tcell.ModNone), // key -> provider
		tcell.NewEventKey(tcell.KeyEsc, "", tcell.ModNone), // provider -> home
		tcell.NewEventKey(tcell.KeyEsc, "", tcell.ModNone), // exit
	})
}

func runTerminalKeyFlow(t *testing.T, keys []*tcell.EventKey) {
	t.Helper()
	previous := ui.DefaultBackend.Screen
	defer func() { ui.DefaultBackend.Screen = previous }()
	terminal := vt.NewMockTerm(vt.MockOptSize{X: 100, Y: 30})
	screen, err := tcell.NewTerminfoScreenFromTty(terminal)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	timedOut := make(chan struct{}, 1)
	err = runWithInitializer(func() error {
		if err := screen.Init(); err != nil {
			return err
		}
		ui.DefaultBackend.Screen = screen
		go func() {
			deadline := time.NewTimer(5 * time.Second)
			defer deadline.Stop()
			for _, event := range keys {
				select {
				case screen.EventQ() <- event:
				case <-done:
					return
				case <-deadline.C:
					timedOut <- struct{}{}
					screen.EventQ() <- tcell.NewEventKey(tcell.KeyCtrlC, "", tcell.ModNone)
					return
				}
			}
			select {
			case <-done:
			case <-deadline.C:
				timedOut <- struct{}{}
				screen.EventQ() <- tcell.NewEventKey(tcell.KeyCtrlC, "", tcell.ModNone)
			}
		}()
		return nil
	})
	close(done)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-timedOut:
		t.Fatal("Escape flow stalled; watchdog had to exit the app")
	default:
	}
}
