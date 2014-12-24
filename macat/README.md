## DESCRIPTION

The **macat** command is a command‐line interface to send and receive data
via the
[mangos](https://github.com/gdamore/mangos) implementation of the SP
[nanomsg](http://www.nanomsg.org) protocols. It is designed to be suitable
for use as a drop‐in replacement for nanocat(1).

Additionally it has a some extra features, to support mangos' additional
capabilities.  In particular, it supports SSL/TLS, and has a few flags
related to that.

## SYNOPSIS
macat <*OPTIONS*>

## OPTIONS

* −v,−−verbose
> Increase verbosity
* −q,−−silent
> Decrease verbosity
* −−push
> Use PUSH socket type
* −−pull
> Use PULL socket type
* −−pub
> Use PUB socket type
* −−sub
> Use SUB socket type
* −−req
> Use REQ socket type
* −−rep
> Use REP socket type
* −−surveyor
> Use SURVEYOR socket type
* −−respondent
> Use RESPONDENT socket type
* −−bus
> Use BUS socket type
* −−pair
> Use PAIR socket type
* −−star
> Use STAR socket type
* −−bind *ADDR*
> Bind socket to *ADDR*
* −−connect *ADDR*
> Connect socket to ADDR
* −X,−−bind‐ipc *PATH*
> Bind socket to IPC PATH
* −x,−−connect‐ipc *PATH*
> Connect socket to IPC *PATH*
* −L,−−bind‐local *PORT*
> Bind socket to TCP localhost *PORT*
* −l,−−connect‐local *PORT*
> Connect socket to TCP localhost *PORT*
* −−subscribe *PREFIX*
> Subcribe to *PREFIX* (default is wildcard)
* −−recv‐timeout *SEC*
> Set receive timeout
* −−send‐timeout *SEC*
> Set send timeout
* −d,−−send‐delay *SEC*
> Set initial send delay
* −−raw
> Raw output, no delimiters
* −A,−−ascii
> ASCII output, one per line
* −Q,−−quoted
> Quoted output, one per line
* −−msgpack
> Msgpacked binay output (see msgpack.org)
* −i,−−interval *SEC*
> Send DATA every *SEC* seconds
* −D,−−data *DATA*
> Data to send
* −F,−−file *FILE*
> Send contents of *FILE*
* −−sslv3
> Force SSLv3 when using SSL/TLS
* −−tlsv1
> Force TLSv1.x when using SSL/TLS
* −−tlsv1.1
> Force TLSv1.0 when using SSL/TLS
* −−tlsv1.1
> Force TLSv1.1 when using SSL/TLS
* −−tlsv1.2
> Force TLSv1.2 when using SSL/TLS
* −E,−−cert *FILE*
> Use certificate in *FILE* for SSL/TLS
* −−key *FILE*
> Use private key in *FILE* for SSL/TLS
* −−cacert *FILE*
> Use CA certicate(s) in *FILE* for SSL/TLS
* −k,−−insecure
> Do not validate TLS/SSL peer certificate
* −−help
> Show usage message

## SSL/TLS DETAILS

The security model for macat is that authentication should be mutual by
default.  Therefore, the user should normally supply both a certificate/key
(either in a single file, or in separate files with *--key*), and a CA 
certificate file (perhaps consisting of many certificates) which should have
been used to sign the peer's certificate.  This is true for **BOTH** client
and server.  (This is a but unlike traditional HTTPS, where it mutual
authentication is the exception rather than the rule.)

A client can choose to skip verification of the server by supplying
the *--insecure* (*-k*) flag instead of the *--cacert* argument.  This is not
recommended.  It can also choose not to authenticate itself to the server
by simply not supplying a *--cert* and *--key*.

The server (the party doing the bind) **MUST** always supply its own
certificate.  It will normally attempt to authenticate clients, but this
can be disabled by supplying *--insecure* or *-k* instead of
the *--cacert* argument.

## AUTHOR
Garrett D’Amore
