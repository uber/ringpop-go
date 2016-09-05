package swim

import (
	"time"

	"github.com/benbjohnson/clock"
)

func schedule(what func(), delayFn func() time.Duration, clock clock.Clock) chan bool {
	stop := make(chan bool)

	go func() {
		for {
			delay := delayFn()
			what()
			select {
			case <-clock.After(delay):
			case <-stop:
				return
			}
		}
	}()

	return stop
}
