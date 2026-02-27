package outbound

import (
	"time"

	ports "neabrain/internal/ports/outbound"
)

// ClockAdapter provides wall-clock timestamps.
type ClockAdapter struct{}

var _ ports.Clock = (*ClockAdapter)(nil)

func NewClockAdapter() *ClockAdapter {
	return &ClockAdapter{}
}

func (c *ClockAdapter) Now() time.Time {
	return time.Now().UTC()
}
