
PLANS
=====

mangos has been quite successful, but over the past several years we've
worked on NNG and nanomsg, as well as mangos, and it's time to consider
some major future work in mangos.  This document describes those plans
in order that everyone who uses mangos can be aware of these.

Note that mangos is developed with Semantic Versioning in mind, so the
changes described here are plans for the next major version (2.0) of mangos.
Users of mangos 1.x can continue to use mangos 1.x for the foreseeable future.

Harmonization with NNG
----------------------

(See github.com/nanomsg/nng to learn more about NNG.)

Mangos has some features (notably the STAR pattern) that NNG lacks, where
NNG has others that mangos lacks (PAIRv1 protocol, contexts, ZeroTier
transport.)  Over time we would like mangos to "acquire" all of the features
that NNG has, and vice versa.  The context support in particular is a huge
boon for folks writing concurrent applications, and as an innovation
We're sorry we didn't think of it during mangos' development.

In addition, some of mangos' terms for concepts are unfortunately different
than those used in nanomsg and NNG.  Much of this stems from our imperfect
understanding of nanomsg architecture when we first wrote mangos.

Finally, mangos suffers in some architectural ways because it is "too simple".
That is to say, we have some patterns that would be better and more efficient
if we could change them to have more knowledge of underlying endpoints, and
give them more visibility into outstanding requests.

Performance
-----------

Mangos is built on top of channels, and uses them heavily, as that was
seen as the most idiomatic way to do things.  Unfortunately, we've seen
demonstrated quite well that Go's channels are pretty poor for certain
use cases, from a performance perspective.  Adding new cases to select
statements, for example, can have a quite measurable impact on performance.

A better, more complex, approach would be to use condition variables and
mutexes a bit more intelligently.  This may be more error prone, and
not something novice developers should do, but we've ample experience with
these constructs from our C and kernel background.  We've long wanted to
fix these in mangos.

Nanomsg Affiliation
-------------------

When we started mangos, mangos was an "outsider" to the nanomsg organization.
We intentionally wanted to avoid creating any false impressions of being
officially blessed by the nanomsg team.  Of course, times have changed,
and we've taken over the nanomsg project.  It's time to bring mangos into
the nanomsg umbrella.  In mangos 2.0, the import path will be moving from
go-mangos (a separate dedicated organization) to nanomsg.org.  Mangos
is *the* officially recommended Scalability Protocols implementation for
Golang applications.  Most likely the import path will become just
nanomsg.org/mangos or possibly nanomsg.org/go-mangos. (We anticipate that
the actual code will continue to be hosted on GitHub, but in light of
recent news about the Microsoft acquisition, it seems like a good time
to stop hard coding GitHub in our package import paths -- the freedom
to depart if things go south at GitHub seems a worthwhile goal.)

Abandoning Compat Package
-------------------------

The compat package was built to make transitioning from op/go-nanomsg
as easy as possible.  (That package is just a native binding to the C
library, and hasn't received any updates in about 2 years.  We think nobody
is really using it anymore.)  We intend to remove the compat package
from mangos as part of the mangos 2.0 effort.

Modern Golang
-------------

There are some compelling reasons to begin requiring Golang 1.8 or newer,
and mangos 2.0 will require it.  For example, net.Buffers will allow us
to make some changes that should substantially improve latency for TCP
based transports.  The WebSocket transport can begin to use the native
WebSocket API in golang now that https://github.com/golang/go/issues/5082
is fixed.  We might begin looking at the native context support that
is present in modern Golang as well.

New Transports
--------------

We expect to bring in transports with support support for QUIC, possibly KCP,
raw UDP, and are contemplating an SSH based transport.  ZMTP is a distinct
possibility in the future as well, as that might lead us to a CurveMQ
transport.  (We have no ill will towards ZeroMQ, we just don't like C++ and
the GPL.)  Folks have expressed an interest in RDMA style protocols as well,
though we would need to have commercial investment to make that happen.

New Protocols
-------------

We have design work for a sender-filtered PUB/SUB protocol, as well as
true multicast delivery, which turns out to be non-obvious in the face
of devices, etc.

There are a number of other patterns which have been discussed, and we
would like to explore these.  Further, some of the existing protocols
can benefit from further hardening against device loops.

Usability Enhancements
----------------------

We think mangos is pretty easy to use, but there are some things that can be
improved upon.  Context support is a big one here, but there are a small
assortment of other ones -- such as easier access to remote pipe credentials
and a better hook mechanism for remote peer connections.

Observability
-------------

It's long been our desire to add deep observability into mangos, starting
first and foremost with rich statistics.  We'd like this to be a key
deliverable for a 2.0 release.

Better Documentation
--------------------

The current documentation is fairly standard for Golang, but we really
want more of a HOWTO guide to building distributed applications with mangos
(and NNG).  A book is planned.

Call for Sponsors
=================

NNG has been fortunate to have received excellent commercial sponsorship,
much of which informs the list of things we want to do here.  However,
in spite of having broader use, mangos has obtained only a small fraction
of the commercial sponsorship that NNG has.  The single best way to help
accelerate any of the above work items is to sponsor their development.
Please contact Staysail Systems, Inc. (info@staysail.tech) to inquire
about sponsoring any of the above items.
