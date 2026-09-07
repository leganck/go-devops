package devops

import "time"

// Clock is an injectable time source (tests, TTL).
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }
