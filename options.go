// Copyright 2014 The Mangos Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use file except in compliance with the License.
// You may obtain a copy of the license at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mangos

// The following are Options used by SetOption, GetOption.

const (
	// OptionRaw is used to enable RAW mode processing.  The details of
	// how this varies from normal mode vary from protocol to protocol.
	// RAW mode corresponds to AF_SP_RAW in the C variant, and must be
	// used with Devices.  In particular, RAW mode sockets are completely
	// stateless -- any state between recv/send messages is included in
	// the message headers.  Protocol names starting with "X" default
	// to the RAW mode of the same protocol without the leading "X".
	// The value passed is a bool.
	OptionRaw = "RAW"

	// OptionRecvDeadline is the absolute time when the next Recv should
	// timeout.  The value is a time.Time.  Zero value may be passed to
	// indicate that no timeout should be applied.  By default there is
	// no timeout is used.
	OptionRecvDeadline = "RECV-DEADLINE"

	// OptionSendDeadline is the absolute time when the next Send should
	// timeout.  The value is a time.Time.  Zero value may be passed to
	// indicate that no timeout should be applied.  By default there is
	// no timeout.
	OptionSendDeadline = "SEND-DEADLINE"

	// OptionRetryTime is used by REQ.  The argument is a time.Duration.
	// When a request has been replied to within the given duration,
	// the request will automatically be resent to an available peer.
	// This value should be longer than the maximum possible processing
	// and transport time.  The value zero indicates that no automatic
	// retries should be sent.  The default value is one minute.
	//
	// Note that changing this option is only guaranteed to affect requests
	// sent after the option is set.  Changing the value while a request
	// is outstanding may not have the desired effect.
	OptionRetryTime = "RETRY-TIME"

	// OptionSubscribe is used by SUB/XSUB.  The argument is a []byte.
	// The application will receive messages that start with this prefix.
	// Multiple subscriptions may be in effect on a given socket.  The
	// application will not receive messages that do not match any current
	// subscriptions.  (If there are no subscriptions for a SUB/XSUB
	// socket, then the application will not receive any messages.  An
	// empty prefix can be used to subscribe to all messages.)
	OptionSubscribe = "SUBSCRIBE"

	// OptionUnsubscribe is used by SUB/XSUB.  The argument is a []byte,
	// representing a previously established subscription, which will be
	// removed from the socket.
	OptionUnsubscribe = "UNSUBSCRIBE"

	// OptionSurveyTime is used to indicate the deadline for survey
	// responses, when used with a SURVEYOR socket.  Messages arriving
	// after this will be discarded.  Additionally, this will set the
	// OptionRecvDeadline when starting the survey, so that attempts to
	// receive messages fail with ErrRecvTimeout when the survey is
	// concluded.  The value is a time.Duration.  Zero can be passed to
	// indicate an infinite time.  Default is 1 second.
	OptionSurveyTime = "SURVEY-TIME"

	// OptionTLSConfig is used to supply TLS configuration details.
	// The parameter is a tls.Config pointer.
	OptionTLSConfig = "TLS-CONFIG"

	// OptionLocalAddress is used to get the local address an accepter is
	// listening on in string form. Generally this is known when Listen is
	// called because it is provided, but this option is useful in the event
	// that the port is assigned by the OS (i.e. port "0").
	OptionLocalAddress = "LOCAL-ADDRESS"

	// OptionWriteQLen is used to set the size, in messages, of the write queue
	// channel. By default, it's 10. This option cannot be set if Dial or
	// Listen has been called on the socket.
	OptionWriteQLen = "WRITEQ-LEN"

	// OptionReadQLen is used to set the size, in messages, of the read queue
	// channel. By default, it's 10. This option cannot be set if Dial or
	// Listen has been called on the socket.
	OptionReadQLen = "READQ-LEN"
)
