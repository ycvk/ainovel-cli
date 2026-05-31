package tui

import "testing"

func TestStreamBufferAppendsIntoCurrentRound(t *testing.T) {
	var buf streamBuffer

	buf.Append("你")
	buf.Append("好")

	got := buf.Snapshot()
	if len(got) != 1 || got[0] != "你好" {
		t.Fatalf("snapshot = %#v, want one combined round", got)
	}
}

func TestStreamBufferStartsNewRoundOnlyAfterContent(t *testing.T) {
	var buf streamBuffer

	buf.StartRound()
	buf.Append("第一轮")
	buf.StartRound()
	buf.StartRound()
	buf.Append("第二轮")

	got := buf.Snapshot()
	if len(got) != 2 || got[0] != "第一轮" || got[1] != "第二轮" {
		t.Fatalf("snapshot = %#v, want two non-empty rounds", got)
	}
}

func TestStreamBufferTrimsOldestRounds(t *testing.T) {
	var buf streamBuffer

	for i := 0; i < maxStreamRounds+2; i++ {
		buf.StartRound()
		buf.Append(string(rune('a' + i)))
	}

	got := buf.Snapshot()
	if len(got) != maxStreamRounds {
		t.Fatalf("len(snapshot) = %d, want %d", len(got), maxStreamRounds)
	}
	if got[0] != "c" {
		t.Fatalf("first retained round = %q, want c", got[0])
	}
}

func TestStreamDeltaSchedulesOneActiveFlush(t *testing.T) {
	m := Model{}

	nextModel, _, handled := m.handleRuntimeMsg(streamDeltaMsg("a"))
	if !handled {
		t.Fatalf("stream delta was not handled")
	}
	next := nextModel.(Model)
	if !next.streamDirty {
		t.Fatalf("streamDirty = false, want true")
	}
	if !next.streamFlushDue {
		t.Fatalf("streamFlushDue = false, want true after first delta")
	}

	nextModel, _, handled = next.handleRuntimeMsg(streamDeltaMsg("b"))
	if !handled {
		t.Fatalf("second stream delta was not handled")
	}
	next = nextModel.(Model)
	if !next.streamFlushDue {
		t.Fatalf("streamFlushDue = false, want still true while tick is pending")
	}
	if got := next.stream.Snapshot(); len(got) != 1 || got[0] != "ab" {
		t.Fatalf("snapshot = %#v, want single buffered round", got)
	}
}

func TestStreamFlushTickStopsWhenIdle(t *testing.T) {
	m := Model{}
	nextModel, _, _ := m.handleRuntimeMsg(streamDeltaMsg("a"))
	next := nextModel.(Model)

	nextModel, _, handled := next.handleRuntimeMsg(streamFlushTickMsg{})
	if !handled {
		t.Fatalf("stream flush tick was not handled")
	}
	next = nextModel.(Model)
	if next.streamDirty {
		t.Fatalf("streamDirty = true, want false after flush")
	}
	if next.streamFlushDue {
		t.Fatalf("streamFlushDue = true, want false after queued tick is consumed")
	}
}
