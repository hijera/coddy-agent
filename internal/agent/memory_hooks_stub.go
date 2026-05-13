//go:build !memory

package agent

import "context"

func (a *Agent) runMemoryBeforeTurn(ctx context.Context, _ string) {}
