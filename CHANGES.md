ringpop-go changes
==================

v0.4.0
------

* Feature: Faulty nodes are now automatically reaped from nodes' membership
  lists after (24 hours by default). #123
* New options for controlling suspect and reaping times. #123
* Add new `Ready` and `Destroyed` events to ringpop. #125
* Add additional logging to bring ringpop-go on par with ringpop-node log
  messages. #116
* Fix bug where Ringpop automatically added itself to the bootstrap hosts for
  host-based bootstrapping, but not other bootstrapping methods. #120
* Fix race condition where membership and hashring could be inconsistent with
  each other. #112
* Remove `File` and `Host` options from bootstrap options in favor of
  `DiscoverProvider` interface. #120
* Add Go 1.6 to testing on CI

### Release notes

#### Version incompatibility in protocol

Since 0.4.0 introduces a new node/member state, 0.4.0 is not backwards-compatible with previous versions.

Note rolling upgrades with older versions do work, but undefined behaviour will occur if two versions run in parallel for longer than the `FaultyPeriod` (default 24 hours).

#### Changes to Bootstrap

This release contains a breaking change to the options provided to the
`ringpop.Bootstrap` call.

`BootstrapOptions.File` and `BootstrapOptions.Hosts` have been replaced with
`BootstrapOptions.DiscoverProvider`. `DiscoverProvider` is an interface which
requires a single method:

```go
type DiscoverProvider interface {
    Hosts() ([]string, error)
}
```

Ringpop comes with DiscoverProviders for the previous `File` and `Hosts`
options out of the box.

To upgrade if you were previously using `File`:

```diff
+       "github.com/uber/ringpop-go/discovery/jsonfile"
...
-       bootstrapOpts.File = *hostfile
+       bootstrapOpts.DiscoverProvider = jsonfile.New(*hostfile)
```

For static `Hosts`:

```diff
+       "github.com/uber/ringpop-go/discovery/statichosts"
...
-       bootstrapOpts.Hosts = []string{"127.0.0.1:3000", "127.0.0.1:3001"}
+       bootstrapOpts.DiscoverProvider = statichosts.New("127.0.0.1:3000", "127.0.0.1:3001")
```


v0.3.0
------

* Fix "keys have diverged" forwarding error for retries #69
* Fix possible race in disseminator #86
* Fix possible issue with leave state not being applied correctly #94
* Fix issues where unnecessary full syncs could occur #95, #97
* Improvements to join:
    * Join retries now have exponential backoff #68
    * Improved resilience to possible partitions at startup #65
    * Reduce network chatter on join #85
* Hashring performance improvements (see discussion on [#58](https://github.com/uber/ringpop-go/pull/58#issuecomment-169653883)) #58
* Revamped logging; new logger options #83
* New stats to aid partition detection #92, #104
* Improved test coverage across the board
* Update and test with latest TChannel (v1.0.3) #103
* Various fixes and improvements to test infrastructure


v0.2.3
------

* Fix retry mechanism for forwarded requests. When multiple keys are forwarded
and a retry is attempted, the retry would fail with a "key destinations have
diverged" error.


v0.2.2
------

* Fix goroutine leakage on forwarded requests that time out


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
