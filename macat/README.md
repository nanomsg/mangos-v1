## macat

macat is a drop in replacement/workalike for nanocat, but based upon mangos
rather than nanomsg.

Additionally it has a some extra features, to support mangos' additional
capabilities.  In particular, it supports SSL/TLS, and has a few flags
related to that.

# SSL/TLS details

The security model for macat is that authentication should be mutual by
default.  Therefore, the user should normally supply both a certificate/key
(either in a single file, or in separate files with --key), and a CA 
certificate file (perhaps consisting of many certificates) which should have
been used to sign the peer's certificate.  This is true for *BOTH* client
and server.  (This is a but unlike traditional HTTPS, where it mutual
authentication is the exception rather than the rule.)

A client can choose to skip verification of the server by supplying
the --insecure (-k) flag instead of the --cacert argument.  This is not
recommended.  It can also choose not to authenticate itself to the server
by simply not supplying a --cert and --key.

The server (the party doing the bind) *MUST* always supply its own
certificate.  It will normally attempt to authenticate clients, but this
can be disabled by supplying --insecure/-k instead of the --cacert argument.
 
