package ringpop

import (
	"errors"
	"fmt"
)

type joinResBody struct {
	App         string
	Coordinator string
	Membership  []Change
}

func validateDenyingJoins(ringpop *Ringpop) error {
	if ringpop.isDenyingJoins {
		return errors.New("Node is currently configured to deny joins.")
	}
	return nil
}

func validateJoinerAddress(ringpop *Ringpop, joiner string) error {
	if joiner == ringpop.WhoAmI() {
		message := fmt.Sprintf(`A node tried joining a cluster by attempting to join itself.
			The joiner (%s) must join someone else.`, joiner)
		return errors.New(message)
	}
	return nil
}

func validateJoinerApp(ringpop *Ringpop, app string) error {
	if app != ringpop.app {
		// revisit message
		message := fmt.Sprintf(`A node tried joining a different app cluster. The
			expected app (%s) did not matcht the actual app (%s)`, ringpop.app, app)
		return errors.New(message)
	}
	return nil
}

func receiveJoin(ringpop *Ringpop, body joinBody) (joinResBody, error) {
	var res joinResBody

	ringpop.stat("increment", "join.recv", 1)

	if err := validateDenyingJoins(ringpop); err != nil {
		return res, err
	}
	if err := validateJoinerAddress(ringpop, body.Source); err != nil {
		return res, err
	}
	if err := validateJoinerApp(ringpop, body.App); err != nil {
		return res, err
	}

	// ringpop.serverRate.Mark(1)
	// ringpop.totalRate.Mark(1)

	ringpop.membership.makeAlive(body.Source, body.Incarnation, "")

	res = joinResBody{
		App:         ringpop.app,
		Coordinator: ringpop.WhoAmI(),
		Membership:  ringpop.dissemination.fullSync(),
	}

	return res, nil
}
