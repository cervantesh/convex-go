// Package baseclient exposes Convex's deterministic sync state machine for
// frameworks, bindings, tests, and alternate transports.
//
// Most applications should use github.com/cervantesh/convex-go directly through
// NewClient. Use this package when you are building a framework, binding, or
// alternate runtime and need to own the transport loop yourself: queue outbound
// sync messages with Client methods, drain them with PopNextMessage, and feed
// inbound server messages through ReceiveMessage.
package baseclient
