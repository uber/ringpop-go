package swim

import (
	"time"

	"github.com/benbjohnson/clock"
)

// TODO rename to scheduleRepeaditly
func schedule(what func(), delayFn func() time.Duration, clock clock.Clock) (stop chan bool, wait <-chan bool) {
	stop = make(chan bool)

	internalWait := make(chan bool)
	wait = internalWait

	go func() {
		defer close(internalWait)
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

	return stop, wait
}

// TODO add schedule once with a quit channel to cancel it for use in the update rollup
