package ai

import (
	"testing"

	genkitAI "github.com/firebase/genkit/go/ai"
)

func TestTurnFromHistoryMatchesStoredTurnSlice(t *testing.T) {
	user := genkitAI.NewUserTextMessage("hi")
	model := genkitAI.NewMessage(genkitAI.RoleModel, nil, genkitAI.NewTextPart("hello"))
	resp := &genkitAI.ModelResponse{
		Request: &genkitAI.ModelRequest{Messages: []*genkitAI.Message{user}},
		Message: model,
	}
	// History() = request messages + response message when present.
	promptLen := 1
	turn := TurnFromHistory(resp, promptLen)
	stored := storedTurn(resp, promptLen)
	if len(turn) != len(stored.Turn) {
		t.Fatalf("turn len %d vs stored %d", len(turn), len(stored.Turn))
	}
	for i := range turn {
		if turn[i].Role != stored.Turn[i].Role || turn[i].Content != stored.Turn[i].Content {
			t.Fatalf("idx %d: turn=%+v stored=%+v", i, turn[i], stored.Turn[i])
		}
	}
}

func TestTurnFromHistoryNil(t *testing.T) {
	if got := TurnFromHistory(nil, 0); got != nil {
		t.Fatalf("got %v", got)
	}
}

func TestTurnFromHistorySkipsSystemAndUser(t *testing.T) {
	sys := genkitAI.NewSystemTextMessage("sys")
	user := genkitAI.NewUserTextMessage("hi")
	model := genkitAI.NewMessage(genkitAI.RoleModel, nil, genkitAI.NewTextPart("hello"))
	resp := &genkitAI.ModelResponse{
		Request: &genkitAI.ModelRequest{Messages: []*genkitAI.Message{sys, user}},
		Message: model,
	}
	// promptLen 0 would leak prompt rows; they must be dropped from the turn.
	turn := TurnFromHistory(resp, 0)
	if len(turn) != 1 || turn[0].Role != RoleModel || turn[0].Content != "hello" {
		t.Fatalf("got %+v", turn)
	}
}
