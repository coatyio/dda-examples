# DDA Example - Partition-Compute-Accumulate

[![Powered by DDA](https://img.shields.io/badge/Powered%20by-DDA-00ADD8.svg)](https://github.com/coatyio/dda)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/coatyio/dda-examples/blob/main/LICENSE)

## Table of Contents

* [Overview](#overview)
* [Quick Start](#quick-start)
* [How It Works](#how-it-works)
  * [Coordinator Logic](#coordinator-logic)
  * [Worker Logic](#worker-logic)
* [Developer Notes](#developer-notes)
* [License](#license)

## Overview

This example project demonstrates how to coordinate large computational
workloads that can be divided into multiple independent partial computations
whose results are combined together to obtain a final result. Partial
computations are distributed to a set of loosely coupled workers using [Data
Distribution Agent](https://github.com/coatyio/dda) (DDA).

This _Partition-Compute-Accumulate_ distribution pattern is realized by two
types of application components:

* `Coordinator` receives a compute request from the command line, splits it into
  partial computations that are dispatched to workers, accumulates all partial
  results into the final result, and yields it.
* `Worker` performs a partial computation requested by a coordinator and returns
  partial result data.

This example supports dynamic setup of coordinator and worker components which
can be manually added to or removed from the distributed system in an ad-hoc
fashion by running executables from the command line. Each coordinator component
coordinates just a single compute request, i.e. it joins the system when
launched and leaves the system after the final result has been yielded. Each
worker joins the system when its launching process is started and runs as long
as this process is not terminated by the user.

This example uses DDA Actions, a two-way communication event pattern, to pass
partial computation data/results between a coordinator and a worker. Each
partial computation is dispatched to exactly one available worker using a DDA
subscription filter which is shared. Workers run concurrently and perform
partial computations one by one, independently of other workers. A worker can
perform computations received by any coordinator which is currently present in
the system.

In addition, this example uses a combination of DDA announcement Events and
Actions to allow a coordinator to dynamically track the workers and other
coordinators which are currently alive. This information is used to scale the
overall data throughput of the distributed system so as to ensure an optimal
utilization of workers avoiding worker overload caused by backpressure during
data transfer. Data throughput is rate limited by coordinators to not exceed the
maximum possible overall throughput: at any time no more partial computations
are dispatched than there are _free_ workers in the system that can handle a
partial computation immediately.

This example utilizes DDA sidecars for coordinator components and the DDA
library for workers. It comes with a predefined set of
_Partition-Compute-Accumulate_ computations:

* `wf` - Distributed text processing of the frequency of occurrence of words in
  a set of documents. The documents are partitioned into paragraphs which are
  evenly distributed among workers that determine the number of occurrences of
  all words.
* `fac` - Distributed mathematical computation of factorial function of a given
  non-negative integer (n!). Useful for demonstration and testing purposes.

Computations are isolated from the application and communication logic and can
be defined separately by implementing an abstract interface definition. Thus,
you can enhance existing ones or add new ones easily.

This project includes a ready-to-use deployment that runs on a variety of
platforms as explained in the next section.

## Quick Start

Download the two coordinator and worker binaries for your platform from the
[GitHub repo](https://github.com/coatyio/dda-examples/tree/main/dist).

Start one DDA sidecar serving all coordinators on your machine and an MQTT 5
broker for DDA communication using one of the following procedures:

* Use predefined Docker compose file to run containers with a DDA sidecar and an
  MQTT 5 broker:

  ```sh
  cd quick-start
  docker compose up

  # To shut down containers
  docker compose down
  ```

* Use one of the prebuilt platform-specific DDA binaries with a default
  configuration file and make an MQTT 5 broker available on your machine. For
  details, see [DDA Quick Start](https://github.com/coatyio/dda#quick-start).

Start a specific number of workers using one of the deployed platform-specific
binaries (for usage details run with `-h` flag):

```sh
cd dist

# Start up 16 worker components
worker_linux_amd64 16
```

Run a coordinator with a desired computation (for usage details run with `-h`
flag):

```sh
cd dist

# Compute word frequency of certain documents in project's testdata folder
coordinator_linux_amd64 wf ../testdata/wf/bavaria-ipsum-*.txt

# Compute 101!
coordinator_linux_amd64 fac 101
```

> __NOTE__: You may also start coordinator or worker components on remote hosts
> to realize a distributed setup with multiple compute nodes. Remote workers
> must use the `-b` command line flag to specify the URL of the MQTT 5 broker
> used by DDA communication service (`-b tcp://mybrokerhost:1883`). Likewise,
> remote coordinators must configure the configuration option `services/com/url`
> of their co-located DDA sidecars. You may also use a single sidecar shared by
> all coordinators in the system.
>
> __TIP__: For debugging and demonstration purposes turn on logging output using
> the `-l` command line flag. It enables logging of the application logic that
> implements the Partition-Compute-Accumulate algorithm.

## How It Works

This example consists of the following main Golang programming entities (for
details see Go doc comments in source code):

* `Coordinator` and `Worker` components implement the overall business logic of
  the Partition-Compute-Accumulate distribution pattern.
* `Computation` is an interface defining the individual functions of the
  Partition-Compute-Accumulate pattern that are invoked by the business logic of
  `Coordinator` and `Worker` components to process a specific compute request.
  Realizations of this interface implement the logic of specific computations
  separate from the overall business logic.
* `Registry` stores concrete computations that implement the `Computation`
  interface. The registry is replicated among coordinators and workers so that
  they can access and invoke the functions of a registered computation.
* `Tracker` collects workers and coordinator instances that are currently
  present in the distributed system. Lifecycle tracking is used by coordinators
  to determine the number of workers that are available for a partial
  computation at hand.

The overall business logic of coordinators and workers treats input and output
data as uninterpreted binary data with a _computation-specific_ encoding. Even
input and output data within a single computation may be encoded differently by
defining individual encodings in the `Partition`, `PartialCompute`, and
`Accumulate` functions.

Note that the Partition-Compute-Accumulate distribution pattern realized in this
example is designed with _stateless_ partial computations in mind. However, the
distribution pattern may be extended to support computation state maintained
either globally, in the worker environment, or in the coordinator environment by
passing state as part of input and output data.

### Coordinator Logic

The overall business logic of a `Coordinator` comprises the following steps:

1. set up a gRPC client connecting to the DDA sidecar communication service,
2. set up distributed lifecycle tracking by subscribing to `announceCoordinator`
   actions published by coordinators when they are joining or leaving the
   system: whenever a joining action by another coordinator is received, this
   coordinator replies so that the joining coordinator can track it,
3. set up distributed lifecycle tracking by subscribing to `announceWorker`
   events published by joining or leaving workers to track them,
4. publish an `announceCoordinator` join action to signal other coordinators and
   workers that this coordinator has joined the system, and await responses by
   them to be tracked,
5. set up the Partition-Compute-Accumulate operations as described below,
6. wait for completion of computation or premature termination signal,
7. publish an `announceCoordinator` leave action to signal other coordinators
   that this coordinator is leaving the system (no reply is awaited),
8. terminate by unsubscribing all DDA subscriptions, and by disconnecting from
   DDA sidecar.

Step 5 processes a given compute request by:

1. invoking the `Partition` function of the corresponding registered computation
   which returns a receive-only channel from which successive input data for
   partial computations can be pulled on demand,
2. repeating the following steps concurrently until all input and output data
   has been processed:
   * reading next input data from the channel _whenever_ a worker is available
     for processing a partial computation,
   * publishing a DDA `partialComputation` action with the read input on the
     shared subscription group `pcompute` established by workers,
   * awaiting output data from ongoing partial computations, and invoking the
     `Accumulate` function of the corresponding registered computation,
3. invoking the `Finalize` function of the corresponding registered computation
   to output the computation result.

If a partial communication cannot be published due to communication errors, or
if an ongoing partial computation yields no output data within a
computation-specific period of time it is scheduled for resubmission. If too
many resubmissions (more than 100) pile up, the coordinator fails fast.

If the returned output data of a partial computation is empty, indicating a
computational error, the coordinator also fails fast.

To determine whether a worker is available for processing a partial computation,
the following algorithm is used: Each coordinator should dispatch a partial
computation at hand not until its current number of ongoing, i.e. published but
not yet completed partial computations is less than the average number of
alive workers _per_ coordinator.

Please note that this uniform distribution logic is just an approximation as the
real availability state of workers is not distributed within the system (for
performance reasons) and thus not known to coordinators. Moreover, a coordinator
cannot directly dispatch a partial computation to a specific worker that is not
busy because workers and coordinators are decoupled by the DDA's underlying
pub-sub system. The implemented approach works best under the assumption that
the pub-sub infrastructure _evenly_ dispatches partial computation requests to
workers on the shared subscription group "pcompute". For example, if MQTT 5
pub-sub protocol is configured in the DDA communication service the installed
MQTT 5 broker should use a round-robin load-balancing strategy on this shared
subscription group. Note that the [emqx](https://www.emqx.io/) broker uses this
distribution mode by default.

An alternative dispatching mechanism to be used instead of a shared subscription
group makes use of a blind auction where the coordinator advertises a partial
computation to all workers which can submit a sealed bid and the best bidder
wins and performs the partial computation. However, the approach taken in this
example is much more performant regarding communication overhead and latency.

### Worker Logic

The overall business logic of a `Worker` comprises the following steps:

1. set up a DDA instance for communication using the DDA library,
2. set up distributed lifecycle tracking by subscribing to `announceCoordinator`
   actions published by coordinators when they are joining or leaving the
   system: whenever a coordinator joining action is received, the worker replies
   so that the joining coordinator can track it.
3. register a shared subscription `pcompute` to receive partial computations
   exclusively and handle them one by one as described below,
4. publish an `announceWorker` join event to signal coordinators that this
   worker has joined the system,
5. wait for termination signal,
6. publish an `announceWorker` leave event to signal coordinators that this
   worker is leaving the system,
7. terminate by unsubscribing all DDA subscriptions, and by closing the DDA
   instance.

Step 3 processes an incoming partial computation by:

1. invoking the `PartialCompute` function of the corresponding registered
   computation, passing in the given input data,
2. publishing the returned output data as a DDA ActionResult.

If a worker cannot perform an incoming partial computation because it is not
registered no response is sent back, eventually causing a computation-specific
timeout to expire on the coordinator side. This triggers resubmission of the
partial computation which is, hopefully, dispatched to another worker that is
capable of processing it.

The partial compute timeout also ensures resubmission in case a partial
computation message is lost in transit.

If a partial computation fails with a computational error or due to invalid
input or output data encoding it returns empty output data causing the receiving
coordinator to fail fast as the partial computation cannot be completed
successfully by any worker.

If a worker cannot perform the given partial computation for other reasons, such
as required resources not being available currently, no response is sent back to
the coordinator, causing resubmission of the unresponsive partial
computation.

## Developer Notes

This example project is organized as a single Go module which consists of
multiple packages that realize functionality of coordinators, workers, and
predefined computations. As a contributor, you may want to add new computations.

To set up the project on your developer machine, install a compatible
[Go](https://go.dev/) version as specified in the `go.mod` file.

Next, install the [Task](https://taskfile.dev/) build tool, either by using one
of the predefined binaries delivered as [GitHub
release](https://github.com/go-task/task/releases) assets or by using the Golang
toolchain:

```sh
go install github.com/go-task/task/v3/cmd/task@latest
```

Run `task install` to install Go tools required to build, test, and release this
module, and to install module dependencies.

All build related tasks can be performed using the Task tool. For a list of
available tasks, run `task`. You may test the system as follows:

```sh
# Start up local containers with a DDA sidecar and an MQTT 5 broker
cd quick-start
docker compose up
```

Next, start workers and coordinators:

```sh
# Start 5 workers with logging enabled
task worker -- -l 5

# Start 5 additional workers in a separate process
task worker -- 5

# Show usage info including predefined computations
task coordinator -- -h

# Run computation fac of 101 (logging enabled)
task coordinator -- -l fac 101

# Run computation wf on test file (logging enabled)
task coordinator -- -l wf testdata/wf/test.txt

# Run computation wf on all go files within this project
task coordinator -- wf ./**/*.go
```

To release your changes, rebuild all binaries (`task bin`) and push your
commits to GitHub.

> __NOTE__: The coordinator component of this example uses the Golang client
> gRPC-Protobuf stubs to access the DDA sidecar communication service. These
> stubs have been copied them from the DDA project (under
> `apis/grpc/stubs/golang`). Alternatively, you may regenerate these stubs using
> a `protoc` compiler with the original DDA Protobuf definitions which are part
> of all DDA release assets.

## License

Code and documentation copyright 2023 Siemens AG.

Code is licensed under the [MIT License](https://opensource.org/licenses/MIT).

Documentation is licensed under a
[Creative Commons Attribution-ShareAlike 4.0 International License](http://creativecommons.org/licenses/by-sa/4.0/).
