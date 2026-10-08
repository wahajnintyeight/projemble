package tui

import "testing"

func TestSessionCommandParsing(t *testing.T) {
	for _, test := range []struct {
		input string
		name  string
		value string
	}{
		{input: "/compact focus on next steps", name: "compact", value: "focus on next steps"},
		{input: "/new carry", name: "new", value: "carry"},
		{input: "/sessions", name: "sessions"},
		{input: "/resume abc12345", name: "resume", value: "abc12345"},
	} {
		command, ok, err := parseSessionCommand(test.input)
		if err != nil || !ok || command.name != test.name || command.value != test.value {
			t.Fatalf("parse %q = %+v, %v, %v", test.input, command, ok, err)
		}
	}
	if _, ok, _ := parseSessionCommand("/absolute/path prompt"); ok {
		t.Fatal("regular slash-prefixed prompt was treated as a session command")
	}
	if _, ok, err := parseSessionCommand("/new unknown"); !ok || err == nil {
		t.Fatal("invalid /new option was accepted")
	}
}
