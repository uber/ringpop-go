# Ringpop-Go
Ringpop brings application-layer sharding to your services in a fault tolerant and scalable manner. It is an embeddable server that reliably partitions your data, detects node failures and easily integrates new nodes into your application cluster when they become available. For more information about the techniques applied within ringpop, see the Concepts section below.

# Table of Contents
* [Motivation](#motivation)
* [Concepts](#concepts)
* [Developer's Guide](#developers-guide)

# Motivation
As an organization's architecture grows in complexity engineers must find a way to make their services more resilient while keeping operational overhead low. ringpop is a step in that direction and an effort to generalize the sharding needs of various services by providing a simple hash ring abstraction. We've found that the use cases to which ringpop can be applied are numerous and that new ones are discovered often.

# Concepts
Ringpop makes use of several techniques to provide applications with a seamless sharding story.

## Gossip
Ringpop implements the SWIM gossip protocol (with minor variations) to maintain a consistent view across nodes within your application cluster. Changes within the cluster are detected and disseminated over this protocol to all other nodes.

## Consistent Hashing
Ringpop leverages consistent hashing to minimize the number of keys that need to be rebalanced when your application cluster is resized. It uses farmhash as its hashing function because it is fast and provides good distribution.

## Proxying
Ringpop offers proxying as a convenience. You may use ringpop to route your application's requests.

## TChannel
TChannel is the transport of choice for ringpop's gossip and proxying capabilities. For more information about TChannel, go here: https://github.com/uber/tchannel.

## Developer's Guide
Coming soon™.
