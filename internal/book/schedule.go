package book

import (
	"context"
	"time"
)

// Future-dated transactions need no job: they are stored like any other transaction
// and simply start counting once their date is today or earlier. The scheduler only
// rebuilds the computed values when the calendar day changes. Recurring bills are
// Kronos's job: it drops them into the inbox on their due date.

// RunScheduler checks every interval whether the day changed and, if so, rebuilds
// the computed values (so a future-dated transaction counts from its date).
func (b *Book) RunScheduler(ctx context.Context, interval time.Duration) {
	b.tick()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			b.tick()
		}
	}
}

func (b *Book) tick() {
	today := b.Today()
	b.mu.RLock()
	dayChanged := b.comp == nil || !b.comp.Today.Equal(today)
	b.mu.RUnlock()
	if dayChanged {
		b.Recompute()
	}
}
