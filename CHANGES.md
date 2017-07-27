ringpop-go changes
==================

v0.8.5
------

* Fix: LookupN returns ring node destinations in ring order [#211](https://github.com/uber/ringpop-go/pull/211)
* Fix: Use lower case sirupsen/logrus import [#208](https://github.com/uber/ringpop-go/pull/208)
* Feature: Added GetNClients to router [#212](https://github.com/uber/ringpop-go/pull/212)

v0.8.4
------

* Allow requiresAppInPing to be configured [#209](https://github.com/uber/ringpop-go/pull/209)

v0.8.3
------

* Fix: Add option to enforce identical app name in pings [#206](https://github.com/uber/ringpop-go/pull/206)

v0.8.2
------

* Fix: Add WithError() to NoLogger [#203](https://github.com/uber/ringpop-go/pull/203)

v0.8.1
------

* Fix: remove error log during bootstrap complaining about not being bootstrapped [#201](https://github.com/uber/ringpop-go/pull/201)

v0.8.0
------

* Fix: ineffectual assignments [#193](https://github.com/uber/ringpop-go/pull/193)
* Fix: Make lookups consistent on hash collisions [#196](https://github.com/uber/ringpop-go/pull/196)
* Feature: Self-eviction [#177](https://github.com/uber/ringpop-go/pull/177)
* Feature: Identity Carry Over [#188](https://github.com/uber/ringpop-go/pull/188) [#189](https://github.com/uber/ringpop-go/pull/189) [#190](https://github.com/uber/ringpop-go/pull/190) [#191](https://github.com/uber/ringpop-go/pull/191) [#192](https://github.com/uber/ringpop-go/pull/192) [#194](https://github.com/uber/ringpop-go/pull/194) [#195](https://github.com/uber/ringpop-go/pull/195)
* Travis: Remove Go 1.5 support [#187](https://github.com/uber/ringpop-go/pull/187)

### Release notes

#### Identity Carry Over
Identity carry over provides a way to manually configure the identity of a member on the hashring.
By changing the identity to be something else than its address, it becomes possible to guarantee ring equality before and after deploys in a dynamic environment (e.g. mesos).

#### Self Eviction
By calling `ringpop.SelfEvict` on shutdown, a member will declare itself as `faulty` and gossips this to the other members.
This removes the time window where it would be marked as `suspect`; in this way removing the time where other members
considered it to be part of the ring while it was not responding anymore.

#### Breaking change to the `identity`-option

Prior to ringpop v0.8.0 the address was used as the identity of a member. Starting with version v0.8.0, it's possible to
configure a separate identity. As a result, the behaviour of the `Identity` and `IdentityResolverFunc` has been changed.
The `Identity` option now configures the identity of a member and will return an error when it matches an ip:port; services
that were using `Identity` or `IdentityResolverFunc` should now use the `Address` and `AddressResolverFunc` options.
You could use the following `gofmt` snippets to easily refactor:

```
gofmt -r 'ringpop.Identity(a) -> ringpop.Address(a)' -w .
gofmt -r 'ringpop.IdentityResolverFunc(a) -> ringpop.AddressResolverFunc(a)' -w .
```

v0.7.1
------

* Fix: Header leaking on the forwarding path caused calls to own endpoints to not be sharded when the same context was used while making the call [#197](https://github.com/uber/ringpop-go/pull/197)

### Release notes

#### Header leaking

When applications use their incoming context blindly in an outgoing call it uses
the same tchannel headers for the outgoing call as the incoming call had,
causing incoming headers to be leaked in the outgoing call. Since most
applications don't use headers there is nothing to worry about. However the
forwarding path of ringpop uses a header to indicate a call has been forwarded.
This header prevents indefinite forwarding loops to occur when applications fail
to converge on ownership.

With this fix the header indicating a forwarded call will not be leaked to the
application. This makes it possible for applications to use the incoming context
on a call to a sharded endpoint on their own or another ringpop servers service
an correctly route the request.

v0.7.0
------

* Feature: Added label support to ringpop for nodes to annotate themselves with more information [#167](https://github.com/uber/ringpop-go/pull/167) [#171](https://github.com/uber/ringpop-go/pull/171) [#172](https://github.com/uber/ringpop-go/pull/172)
* Maintainability: Refactored internal representation of member state for more flexible reincarnation and state change of a member [#159](https://github.com/uber/ringpop-go/pull/159) [#161](https://github.com/uber/ringpop-go/pull/161)
* Fix: Make sure ringpop is listening for requests before bootstrapping [#176](https://github.com/uber/ringpop-go/pull/176) [#146](https://github.com/uber/ringpop-go/pull/146)
* Fix: Added support for imports in `.thrift` files in generated code for thrift forwarding [#162](https://github.com/uber/ringpop-go/pull/162)
* Fix: Mark generated code as being generated for suppression in diffs [#169](https://github.com/uber/ringpop-go/pull/169)
* Fix: Be more specific in the functionality required from TChannel [#166](https://github.com/uber/ringpop-go/pull/166)
* Tests: Run all tests on go 1.7 [#179](https://github.com/uber/ringpop-go/pull/179)
* Tests: More stable unit tests, integration tests and race detector tests. Races are now mandatory tests on ci [#164](https://github.com/uber/ringpop-go/pull/164) [#178](https://github.com/uber/ringpop-go/pull/178) [#181](https://github.com/uber/ringpop-go/pull/181) [#182](https://github.com/uber/ringpop-go/pull/182)
* Tests: All examples are tested on every pull request [#157](https://github.com/uber/ringpop-go/pull/157) [#170](https://github.com/uber/ringpop-go/pull/170)

### Release notes

#### Change to ringpop interface

The ringpop interface changed two existing functions `GetReachableMembers` and
`CountReachableMembers` that now take a variadic argument of type
`swim.MemberPredicate` instead of no arguments. This does not change the usage
of these functions, but does change the type of the function. This might cause
custom declared interfaces to not match ringpop anymore. The solution is to
change these functions in the interface used to match the current signature.

Previously the signature was:
```golang
  GetReachableMembers() ([]string, error)
  CountReachableMembers() (int, error)
```

The current signature is:
```golang
  GetReachableMembers(predicates ...swim.MemberPredicate) ([]string, error)
  CountReachableMembers(predicates ...swim.MemberPredicate) (int, error)
```

#### Deprecated RegisterListener

Due to a refactor in how event emitting is done the `RegisterListener` method is
deprecated. Even though it still works and behaves as previously it will start
logging warnings. Since this code is not on the hot path only little log volume
is expected. Instead of this function it is now advised to use `AddListener`.
This function also returns if the listener has been added or not.

v0.6.0
------------

* Fix: add 1-buffer for error channels, prevents leaking goroutines. #140
* Feature: support forwarding headers. #141
* Feature: testpop takes an additional -stats-udp or -stats-file flag that can
  be used to emit statsd messages to a file or over the network. #144
* Fix: race condition when loading hostlist in discover provider. #145
* Testing: add suspect, faulty and tombstope period command-line flags to
  testpop. #148
* Fix: use oneself as a source when creating a change for reincarnation. #150
* Maintainability: move from godep to Glide. #151
* Maintainability: emit lookup and lookupn stats and add a lookupn-event. #152.
* Fix: do not reincarnate twice unnecessarily. #153

v0.5.0
------

* Feature: Automatic healing of fully partitioned rings that could be cause by
  temporary network failure. #135 #128 #129
* Fix: Fullsyncs are now executed in both directions to prevent persistent
  asymmetrical partitions. #134
* Fix: Overcome protocol incompatibilities in the dissemination of tombstones
  for the reaping of faulty nodes. #133 #132
* Fix: Leaking goroutines on an error response after a timeout. #140
* Maintainability: Refactor of internal use of the Discovery Provider. #130
* Testing: Pin mockery to a specific version for testing stability. #136
* Testing: Add Go race detector on CI. #137

### Release notes

#### Deploying from older versions

It is advised to complete the deployment of this version within 24 hours after
the first node is upgraded. Since the reaping of faulty nodes is added to this
release a node that upgraded to this version will mark all the faulty members of
the ringpop cluster as tombstones (the new state introduced for the reaping of
faulty nodes) 24 hours after the deploy. If older versions of ringpop run in a
cluster that starts declaring these `tombstones` the cluster will jump in endless
fullsyncs mode. Ringpop operates normally under these conditions but extra load
on both network and CPU are in effect during this time. The fullsyncs will
automatically resolve as soon as all members are upgraded to this version. Same
might happen during a partial rollback that operates for a longer period of time.

v0.4.1
------

* Reverting reaping faulty nodes feature temporarily while investigate backwards
  compatibility issues.

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
