## mangos <img src=mangos.jpg align=right>


[![Linux Status](https://img.shields.io/travis/go-mangos/mangos.svg?label=linux)](https://travis-ci.org/go-mangos/mangos)
[![Windows Status](https://img.shields.io/appveyor/ci/gdamore/mangos.svg?label=windows)](https://ci.appveyor.com/project/gdamore/mangos)
[![Apache License](https://img.shields.io/badge/license-APACHE2-blue.svg)](https://github.com/go-mangos/mangos/blob/master/LICENSE)
[![Gitter](https://img.shields.io/badge/gitter-join-brightgreen.svg)](https://gitter.im/gdamore/mangos)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://godoc.org/github.com/go-mangos/mangos)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-mangos/mangos)](https://goreportcard.com/report/github.com/go-mangos/mangos)

Package mangos is an implementation in pure Go of the SP
("Scalability Protocols")
messaging system.
This makes heavy use of go channels, internally, but it can operate
on systems that lack support for cgo.

The reference implementation of the SP protocols is available as
[nanomsg&trade;](http://www.nanomsg.org); there is also an effort to implement
an improved and more capable version of nanomsg called
[NNG&trade;](https://github.com/nanomsg/nng).
 
The design is intended to make it easy to add new transports with almost trivial
effort, as well as new topologies ("protocols" in SP terminology.)

At present, all of the Req/Rep, Pub/Sub, Pair, Bus, Push/Pull, and
Surveyor/Respondent patterns are supported.

Additionally, there is an experimental new pattern called STAR available.  This
pattern is like Bus, except that the messages are delivered not just to
immediate peers, but to all members of the topology.  Developers must be careful
not to create cycles in their network when using this pattern, otherwise
infinite loops can occur.

Supported transports include TCP, inproc, IPC, Websocket, Websocket/TLS and TLS.
Use addresses of the form "tls+tcp://<host>:<port>" to access TLS.
Note that ipc:// is not supported on Windows (by either this or the reference
implementation.)  Forcing the local TCP port in Dial is not supported yet (this
is rarely useful).

Basic interoperability with nanomsg and NNG has been verified (you can do
so yourself with nanocat and macat) for all protocols and transports
that NNG and nanomsg support.
Additionally there are a number of projects that use the two products together.

There is a third party experimental QUIC transport available at
[quic-mangos](https://github.com/lthibault/quic-mangos).  (An RFE to make this
transport official exists.)

If you find this useful, I would appreciate knowing about it.  I can be reached
via my email address, garrett -at- damore -dot- org

## Installing

### Using *go get*

    $ go get -u github.com/go-mangos/mangos

After this command *mangos* is ready to use. Its source will be in:

    $GOPATH/src/pkg/github.com/go-mangos/mangos

You can use `go get -u -a` to update all installed packages.

## Documentation

For docs, see http://godoc.org/github.com/go-mangos/mangos or run:

    $ godoc github.com/go-mangos/mangos

## Testing

This package supports internal self tests, which can be run in
the idiomatic Go way.  (Note that most of the tests are in a test
subdirectory.)

    $ go test github.com/go-mangos/mangos/...

There are also internal benchmarks available:

    $ go test -bench=. github.com/go-mangos/mangos/test

## Commercial Support

[Staysail Systems, Inc.](mailto:info@staysail.tech) offers
[commercial support](http://staysail.tech/support/mangos) for mangos.

## Examples

Some examples are posted in the directories under examples/
in this project.

These examples are rewrites (in Go) of Tim Dysinger's 
[Getting Started with Nanomsg](http://nanomsg.org/gettingstarted/index.html).

godoc in the example directories will yield information about how to run
each example program.

Enjoy!

Copyright 2017 The Mangos Authors

Nanomsg&trade; and NNG&trade; are [trademarks](http://nanomsg.org/trademarks.html) of Garrett D'Amore.
