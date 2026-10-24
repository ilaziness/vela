package ai

import (
	"context"

	genkitAI "github.com/firebase/genkit/go/ai"
)

// checkpointMiddleware snapshots History[promptLen:] after each generate for durable persistence.
type checkpointMiddleware struct {
	promptLen    int
	onCheckpoint func([]ChatMessage)
}

func (c checkpointMiddleware) Name() string { return "vela/checkpoint" }

func (c checkpointMiddleware) New(ctx context.Context) (*genkitAI.Hooks, error) {
	_ = ctx
	if c.onCheckpoint == nil {
		return &genkitAI.Hooks{}, nil
	}
	promptLen := c.promptLen
	onCheckpoint := c.onCheckpoint
	return &genkitAI.Hooks{
		WrapGenerate: func(ctx context.Context, params *genkitAI.GenerateParams, next genkitAI.GenerateNext) (*genkitAI.ModelResponse, error) {
			resp, err := next(ctx, params)
			if resp != nil {
				if turn := TurnFromHistory(resp, promptLen); len(turn) > 0 {
					onCheckpoint(turn)
				}
			}
			return resp, err
		},
	}, nil
}
