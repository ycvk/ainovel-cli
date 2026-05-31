package tui

import (
	"strings"
)

type streamBuffer struct {
	rounds []*strings.Builder
}

func (b *streamBuffer) Append(text string) {
	if text == "" {
		return
	}
	b.current().WriteString(text)
}

func (b *streamBuffer) StartRound() {
	if len(b.rounds) == 0 {
		b.rounds = append(b.rounds, &strings.Builder{})
		return
	}
	if strings.TrimSpace(b.rounds[len(b.rounds)-1].String()) != "" {
		b.rounds = append(b.rounds, &strings.Builder{})
	}
	b.Trim(maxStreamRounds)
}

func (b *streamBuffer) Reset() {
	b.rounds = nil
}

func (b *streamBuffer) Trim(maxRounds int) {
	if maxRounds <= 0 || len(b.rounds) <= maxRounds {
		return
	}
	drop := len(b.rounds) - maxRounds
	b.rounds = b.rounds[drop:]
}

func (b *streamBuffer) Snapshot() []string {
	if len(b.rounds) == 0 {
		return nil
	}
	out := make([]string, len(b.rounds))
	for i, round := range b.rounds {
		out[i] = round.String()
	}
	return out
}

func (b *streamBuffer) current() *strings.Builder {
	if len(b.rounds) == 0 {
		b.rounds = append(b.rounds, &strings.Builder{})
	}
	return b.rounds[len(b.rounds)-1]
}
