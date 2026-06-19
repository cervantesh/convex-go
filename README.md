# Convex

Community Go client for [Convex](https://convex.dev/).

This module is community-maintained, pre-v1, and not an official first-party
Convex client.

## Installation

```sh
go get github.com/cervantesh/convex-go
```

## Example

```go
package main

import (
	"context"
	"log"
	"os"

	convex "github.com/cervantesh/convex-go"
)

func main() {
	ctx := context.Background()
	client, err := convex.NewClient(os.Getenv("CONVEX_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	if _, err := client.Query(ctx, "messages:list", map[string]any{
		"limit": convex.Number(10),
	}); err != nil {
		log.Fatal(err)
	}

	if _, err := client.Mutation(ctx, "messages:send", map[string]any{
		"body": "Hello from Go",
	}); err != nil {
		log.Fatal(err)
	}

	subscription, err := client.Subscribe(ctx, "messages:list", nil)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = subscription.Close() }()

	if _, err := subscription.Next(ctx); err != nil {
		log.Fatal(err)
	}
}
```

## Documentation

- [docs/USAGE.md](docs/USAGE.md) for normal use with `convex.NewClient`,
  auth, errors, values, pagination, and explicit `NewHTTPClient` /
  `NewWebSocketClient` flows.
- [docs/PARITY.md](docs/PARITY.md) for supported surface and current limits.
- [docs/COMPATIBILITY.md](docs/COMPATIBILITY.md) for supported Go versions,
  CI matrix, and live-backend evidence.
- [docs/CONFORMANCE.md](docs/CONFORMANCE.md) for offline compatibility
  fixtures and upstream sources.
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for package boundaries and the
  advanced `baseclient` path.
- [docs/MAINTAINERS.md](docs/MAINTAINERS.md) for contributor, quality, and
  release workflow docs.
- [pkg.go.dev](https://pkg.go.dev/github.com/cervantesh/convex-go) for the
  generated Go API reference.

## Supported Go Version

This module supports Go 1.25 or newer.

## Community / Support

- [SUPPORT.md](SUPPORT.md) covers SDK support scope and issue routing.
- [SECURITY.md](SECURITY.md) covers private security reporting.
- [CONTRIBUTING.md](CONTRIBUTING.md) covers repository workflow details.
- Join the [Convex community](https://www.convex.dev/community) for broader
  Convex discussion.
