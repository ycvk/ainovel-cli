package host

import (
	"fmt"

	"github.com/voocel/ainovel-cli/internal/domain"
)

func (h *Host) emitDurableEvent(ev Event) error {
	if err := h.persistEvent(ev); err != nil {
		return err
	}
	h.emitEvent(ev)
	return nil
}

func (h *Host) persistEvent(ev Event) error {
	if h.store == nil || h.store.Runtime == nil {
		return nil
	}
	if _, err := h.store.Runtime.AppendQueue(runtimeQueueItemFromEvent(ev)); err != nil {
		return fmt.Errorf("append runtime queue event %q: %w", ev.Summary, err)
	}
	return nil
}

func runtimeQueueItemFromEvent(ev Event) domain.RuntimeQueueItem {
	return domain.RuntimeQueueItem{
		Time:     ev.Time,
		Kind:     domain.RuntimeQueueUIEvent,
		Priority: runtimeQueuePriorityForEvent(ev),
		Agent:    ev.Agent,
		Category: ev.Category,
		Summary:  ev.Summary,
		Payload:  ev,
	}
}

func runtimeQueuePriorityForEvent(ev Event) domain.RuntimeQueuePriority {
	switch ev.Category {
	case "SYSTEM", "ERROR", "USER":
		return domain.RuntimePriorityControl
	default:
		return domain.RuntimePriorityBackground
	}
}
