# Consistency with ZedTokens

## Overview

In SpiceDB, resources are arranged in a graph where different services may be contributing to that graph. For example, permissions-api itself manages roles and role bindings, while tenant-api manages tenants. Resources are updated over NATS request/reply.

SpiceDB breaks updates down into quantized revisions of configurable duration, meaning ACL updates within a given revision window are not visible by default until the next revision begins. Using ZedTokens, however, clients can request data at least as fresh as a given exact point in time. This allows for clients to control the consistency they need for a given SpiceDB request on a per-request basis.

In general, services expect that immediately after updating relationships in permissions-api, they can make authorization decisions based on data at least as fresh as when they made those updates.

## Goals

This proposal is meant to achieve the following goals:

* Permissions checks made on a resource after it is updated must use data at least as fresh as the last update to that resource

## Non-goals

This proposal is not meant to achieve the following goals:

* Updates to the graph are globally and immediately propagated

## Proposed solution

To mitigate this issue, permissions-api can be updated to use a table in CRDB to populate ZedTokens for recently updated resources. By doing this, permissions-api becomes responsible for determining the freshness of lookups. On permissions checks for a given resource, the following consistency strategy is used:

* If a ZedToken exists for that resource, use `at_least_as_fresh` with the given ZedToken
* If no ZedToken exists, use `minimize_latency`

The advantage of this approach is that it keeps management of consistency within permissions-api, meaning future changes do not (necessarily) require updates to other services, nor does it change current API semantics. Additionally, it does not introduce new dependencies to permissions-api, instead leveraging CRDB's availability and fault tolerance guarantees.

## Constraints and limitations

As designed, this only provides immediately visible updates for a given resource; other indirectly affected resources are not updated (but could be in the future). Additionally, given CRDB leaseholder behavior, multi-region deployments may see longer durations when performing permissions checks if the ZedToken table is not a global table.

## Alternatives considered

### Stream ZedToken updates using the SpiceDB Watch API

We could minimize lookup time for ZedTokens by using the SpiceDB Watch API and streaming updates to all permissions-api replicas. However, this means updates will not be immediately visible for a given resource globally. While tokens could be stored in clients (i.e., using the IAM runtime), this merely pushes the complexity of managing ZedTokens to downstream services.

### Store ZedTokens using NATS KV

ZedTokens could be stored using KV in NATS as a coordination mechanism between servers globally. NATS clusters are in general meant to have very low latency between servers, where a single cluster coordinates a given stream (and thus a given KV bucket). This means that for immediate updates to resources using KV, all permissions-api replicas either need to read from a single NATS cluster (introducing latency as a function of distance from the cluster) or use a large cluster that spans many regions. The latter approach is [explicitly discouraged][nats-discussion] by NATS maintainers.

[nats-discussion]: https://github.com/nats-io/nats-server/discussions/5317#discussioncomment-9138192
