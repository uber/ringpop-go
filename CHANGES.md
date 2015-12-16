ringpop-go changes
==================

v0.2
----

* Add Thrift forwarding support #31
* Add `Ringpop.Ready()` #32
* Add `Ringpop.GetReachableMembers()` and `Ringpop.CountReachableMembers()` #29
* Add `Ringpop.Forward()` #26
* Lazy initialization and identity autodetection #40
* Improve stats and bring them in-line with those emitted by ringpop-node #46
* Automatically add self to bootstrap list #41
* Improve HashRing API #38
* Improve tests #35
* Improve constructor options pattern for Ringpop #33
* Disable TChannel retries and zipkin tracing #28
* Fix headers sent with forwarded requests #22

### Release notes

There are a significant number of breaking changes in this release:

* Ringpop constructor has been renamed from `Ringpop.NewRingpop` to `ringpop.New(app string, opts ...Option)` and now accepts optional functional arguments. See the [package documentation](https://godoc.org/github.com/uber/ringpop-go#Option) for a list of options.
* `ringpop.Bootstrap` now accepts `swim.BootstrapOptions`. `ringpop.BootstrapOptions` has been removed.
* Many public methods now return an error if they are called before the ring is bootstrapped. Signatures for the changed methods are:
    * `Bootstrap(opts *swim.BootstrapOptions) ([]string, error)`
    * `Checksum() (uint32, error)`
    * `Lookup(key string) (string, error)`
    * `LookupN(key string, n int) ([]string, error)`
    * `Uptime() (time.Duration, error)`
    * `WhoAmI() (string, error)`
* Removed public method `ringpop.Destroyed()`. Use the new `ringpop.Ready()` to determine if the ring is ready or not.
* Events moved to `ringpop.events` package. This includes:
    * `EventListener`
    * `RingChangedEvent`
    * `RingChecksumEvent`
    * `LookupEvent`
* The behaviour of creating a single-node cluster has changed. Calling `Bootstrap()` with no bootstrap file or hosts will now cause Ringpop to create a single-node cluster.

v0.1
----

* Initial public release
