// Package convex provides a Community Go client for Convex deployments.
//
// This module is not an official first-party Convex client. It is designed for
// Go applications that need to call Convex queries, mutations, actions, and
// realtime sync subscriptions.
//
// Primary APIs:
//   - NewClient creates the main client facade for Query, Mutation, Action,
//     Subscribe, and Close.
//   - NewHTTPClient creates an explicit HTTP-only client.
//   - NewWebSocketClient creates an explicit realtime client for Subscribe and
//     sync Mutation calls.
//
// Typed and generated APIs:
//   - NewQueryReference, NewMutationReference, and NewActionReference provide
//     typed call sites.
//   - cmd/convex-go-codegen can generate reference declarations from local
//     Convex source files.
//
// Realtime APIs:
//   - Client.Subscribe starts normal application query subscriptions.
//   - WebSocketClient.Subscribe starts query subscriptions.
//   - WebSocketClient.WatchAll is an advanced helper for coalesced snapshots of
//     active query results.
//
// Advanced protocol APIs:
//   - github.com/cervantesh/convex-go/baseclient exposes the deterministic
//     sync state machine for framework integrations and protocol work.
//   - The root package keeps a small pre-v1 compatibility wrapper set for
//     advanced sync auth tokens, but new auth callback and protocol-facing
//     work should prefer package baseclient directly.
//   - Raw wire messages, codecs, reconnect loops, and the low-level WebSocket
//     manager live under internal packages. Most applications should prefer
//     Client, HTTPClient, and WebSocketClient.
package convex
