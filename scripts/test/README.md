# TEST USAGE

First `go build ./...`

Then `./test -n x` where x is the number of ringpop instances to start on execution of the program.

During execution the following commands are available to you:

* `s` `hostport` -- starts a ringpop on the specified host:port.
* `sn` `n` -- starts n ringpop instances, prioritizing those that were killed previously
* `k` `hostport` -- kills the ringpop on the specified host:port.
* `kn` `n` -- kills n random ringpops
