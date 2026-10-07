package cli

import (
	"context"
	"strings"
	"testing"

	"projemble/internal/agent"
)

func TestTerminalPermissionApproverAllowsOnlyExplicitYes(t *testing.T) {
	for _, test := range []struct {
		input string
		allow bool
	}{
		{input: "yes\n", allow: true},
		{input: "y\n", allow: true},
		{input: "no\n", allow: false},
		{input: "\n", allow: false},
	} {
		var output strings.Builder
		approver := newTerminalPermissionApprover(strings.NewReader(test.input), &output)
		allow, err := approver.RequestPermission(context.Background(), agent.PermissionRequest{Action: "Read project file", Target: "README.md"})
		if err != nil || allow != test.allow {
			t.Errorf("approval for %q = %v, err=%v; want %v", test.input, allow, err, test.allow)
		}
		if !strings.Contains(output.String(), "README.md") {
			t.Errorf("approval prompt omitted target: %q", output.String())
		}
	}
}
