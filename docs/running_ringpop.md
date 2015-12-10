## Monitoring
Ringpop emits stats by making use of the dependency it has on a Statsd-
compatible client. It emits all stats with a prefix that includes its
identity in the stat path, e.g. `ringpop.10_30_8_26_20600.*`; the dots
and colon are replaced by underscores. The table below lists all stats
that Ringpop emits:

|Node.js Path|Description|Type|Golang
|----|----|----|----
|changes.apply|Number of changes applied per membership update|gauge|Yes
|changes.disseminate|Number of changes disseminated per request/response|gauge|Yes
|checksum|Value of membership checksum|gauge|Yes
|compute-checksum|Time required to compute membership checksum|timer|Yes
|damp-req.recv|Damp-req request received|count|N/A
|damp-req.send|Damp-req request sent|count|N/A
|damper.damp-req.damped|Damp-req resulted in members being damped|count|N/A
|damper.damp-req.error|Damp-req resulted in an error|count|N/A
|damper.damp-req.inconclusive|Damp-req results were inconclusive|count|N/A
|damper.flapper.added|Flap damping detected a flappy node|count|N/A
|damper.flapper.removed|Flap damping removed a flappy node|count|N/A
|damper.flappers|Number of current flappers|gauge|N/A
|filtered-change|A change to be disseminated was deduped|count|Yes
|full-sync|Number of full syncs transmitted|count|Yes
|join.complete|Join process completed successfully|count|Yes
|join.failed.destroyed|Join process failed because Ringpop had been destroyed|count|Yes
|join.failed.err|Join process failed because of an error|count|Yes
|join.recv|Join request received|count|Yes
|join.retries|Number of retries required by join process|gauge|Yes
|join.succeeded|Join process succeeded|count|Yes
|join|Time required to complete join process successfully|timer|Yes
|lookup|Time required to perform a ring lookup|timer|Yes
|make-alive|A member was declared alive|count|Yes
|make-damped|A member was declared damped|count|N/A
|make-faulty|A member was declared faulty|count|Yes
|make-leave|A member was declared leave|count|Yes
|make-suspect|A member was declared suspect|count|Yes
|max-piggyback|Value of the max piggyback factor|gauge|Yes
|membership-set.alive|A member was initialized in the alive state|count|Yes
|membership-set.faulty|A member was initialized in the faulty state|count|Yes
|membership-set.leave|A member was initialized in the leave state|count|Yes
|membership-set.suspect|A member was initialized in the suspect state|count|Yes
|membership-set.unknown|A member was initialized in an unknown state|count|Yes
|membership-update.alive|A member was updated to be alive|count|Yes
|membership-update.faulty|A member was updated to be faulty|count|Yes
|membership-update.leave|A member was updated in the leave state|count|Yes
|membership-update.suspect|A member was updated to be suspect|count|Yes
|membership-update.unknown|A member was updated in the unknown state|count|Yes
|membership.checksum-computed|Membership checksum was computed|count|Yes
|not-ready.ping-req|Ping-req received before Ringpop was ready|count|-
|not-ready.ping|Ping received before Ringpop was ready|count|-
|num-members|Number of members in the membership|gauge|-
|ping-req-ping|Indirect ping sent|timer|Yes
|ping-req.other-members|Number of members selected for ping-req fanout|timer|-
|ping-req.recv|Ping-req request received|count|Yes
|ping-req.send|Ping-req request sent|count|Yes (validate same count)
|ping-req|Ping-req response time|timer|Yes
|ping.recv|Ping request received|count|Yes
|ping.send|Ping request sent|count|Yes
|ping|Ping response time|timer|Yes
|protocol.damp-req|Damp-req response time|timer|-
|protocol.delay|How often gossip protocol is expected to tick|timer|Yes
|protocol.frequency|How often gossip protocol actually ticks|timer|Yes
|refuted-update|A member refuted an update for itself|count|-
|requestProxy.checksumsDiffer|Checksums differed when a forwarded request was received|count|-
|requestProxy.egress|Request was forwarded|count|-
|requestProxy.inflight|Number of inflight forwarded requests|gauge|-
|requestProxy.ingress|Forward request was received|count|-
|requestProxy.miscount.decrement|Number of inflight requests were miscounted after decrement|count|-
|requestProxy.miscount.increment|Number of inflight requests were miscounted after increment|count|-
|requestProxy.refused.eventloop|Request was refused due to event loop lag|count|-
|requestProxy.refused.inflight|Request was refused due to number of inflight requests|count|-
|requestProxy.retry.aborted|Forwarded request retry was aborted|count|-
|requestProxy.retry.attempted|Forwarded request retry was attempted|count|-
|requestProxy.retry.failed|Forwarded request failed after retries|count|-
|requestProxy.retry.reroute.local|Forwarded request retry was rerouted to local node|count|-
|requestProxy.retry.reroute.remote|Forwarded request retry was rerouted to remote node|count|-
|requestProxy.retry.succeeded|Forwarded request succeeded after retries|count|-
|requestProxy.send.error|Forwarded request failed|count|-
|requestProxy.send.success|Forwarded request was successful|count|-
|ring.change|Hash ring keyspace changed|gauge|-
|ring.checksum-computed|Hash ring checksum was computed|count|Yes
|ring.server-added|Node (and its points) added to hash ring|count|Yes
|ring.server-removed|Node (and its points) removed from hash ring|count|Yes
|updates|Number of membership updates applied|timer|-