## sp

[![GoDoc](https://godoc.org/bitbucket.org/gdamore/sp?status.png)](https://godoc.org/bitbucket.org/gdamore/sp)

Package sp is an implementation in pure Go of the SP ("Scalable Protocols")
protocols.  This makes heavy use of go channels, internally, but it can
operate on systems that lack support for cgo.  It has no external dependencies.

The reference implementation of the SP protocols is available as
[nanomsg](http://www.nanomsg.org)
 
The design is intended to make it easy to add new transports with almost
trivial effort, as well as new topologies ("protocols" in SP terminology.)

At present, only Req/Rep, Pub/Sub, and Pair over TCP, IPC, and TLS are
supported. TLS support is experimental.  Use addresses of the form
"tls+tcp://<host>:<port>" to access it.  Note that ipc:// is not supported
on Windows (by either this or the reference implementation.)  Forcing the local
TCP port in Dial is not supported yet (this is rarely useful).

Basic interoperability with nanomsg has been tested with nanocat.

Consider this a work-in-progress, and use at your own risk.

If you find this useful, I would appreciate knowing about it.  I can be reached
via my email address, garrett -at- damore -dot- org

Apologies for the crummy name.  If anyone has a snappy name for this
implementation, I'd like to hear about it.

## Installing

### Using *go get*

    $ go get bitbucket.org/gdamore/sp

After this command *sp* is ready to use. Its source will be in:

    $GOPATH/src/pkg/bitbucket.org/gdamore/sp

You can use `go get -u -a` to update all installed packages.

## Documentation

For docs, see http://godoc.org/bitbucket.org/gdamore/sp or run:

    $ godoc bitbucket.org/gdamore/sp

## Testing

This package supports internal self tests, which can be run in
the idiomatic Go way:

    $ go test bitbucket.org/gdamore/sp

There are also internal benchmarks available:

	$ go test -v -bench=. bitbucket.org/gdamore/sp

Enjoy!