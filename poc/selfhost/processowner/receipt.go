package processowner

import "context"

// JoinAfterReceipt is only for a pinned one-shot child whose trusted terminal
// receipt was already consumed and validated. EOF releases only its reader;
// zero exit and no extra frames remain mandatory. It sends no new operation.
func (c *Child) JoinAfterReceipt(ctx context.Context) error { return c.finish(ctx, nil, 4) }
func (c *Child) PID() int {
	if c == nil || c.cmd == nil || c.cmd.Process == nil {
		return 0
	}
	return c.cmd.Process.Pid
}
