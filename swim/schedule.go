package swim

import (
	"time"

	"github.com/benbjohnson/clock"
)

func schedule(what func(), delayFn func() time.Duration, clock clock.Clock) chan bool {
	stop := make(chan bool)

	go func() {
		for {
			what()
			select {
			case <-clock.After(delayFn()):
			case <-stop:
				return
			}
		}
	}()

	return stop
}
