package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"projemble/internal/agent"
)

type terminalPermissionApprover struct {
	input  *bufio.Reader
	output io.Writer
}

type terminalPermissionResult struct {
	answer string
	err    error
}

func newTerminalPermissionApprover(input io.Reader, output io.Writer) terminalPermissionApprover {
	return terminalPermissionApprover{input: bufio.NewReader(input), output: output}
}

func (approver terminalPermissionApprover) RequestPermission(ctx context.Context, request agent.PermissionRequest) (bool, error) {
	if _, err := fmt.Fprintf(approver.output, "\n%s\nTarget: %s\nAllow this action once? [y/N] ", request.Action, request.Target); err != nil {
		return false, err
	}
	result := make(chan terminalPermissionResult, 1)
	go func() {
		answer, err := approver.input.ReadString('\n')
		result <- terminalPermissionResult{answer: answer, err: err}
	}()
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case input := <-result:
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if input.err != nil && !errors.Is(input.err, io.EOF) {
			return false, input.err
		}
		return strings.EqualFold(strings.TrimSpace(input.answer), "y") || strings.EqualFold(strings.TrimSpace(input.answer), "yes"), nil
	}
}
