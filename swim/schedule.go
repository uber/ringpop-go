package swim

import (
	"time"

	"github.com/benbjohnson/clock"
)

// scheduleRepeaditly runs a function in the background continually with a
// dynamically computed delay in between invocations. The returned channels can
// be used to stop the background execution and a channel that will be unblocked
// when the background task is completely stopped.
//
//   stop, wait := scheduleRepeaditly(func(){..}, func() time.Duration { return time.Second }, clock)
//   time.sleep(time.Duration(10) * time.Second)
//   // stop the background task
//   close(stop)
//   // wait till the background task is terminated
//   <-wait
func scheduleRepeaditly(what func(), delayFn func() time.Duration, clock clock.Clock) (stop chan bool, wait <-chan bool) {
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
