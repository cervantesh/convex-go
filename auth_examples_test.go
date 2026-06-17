package convex_test

import (
	"context"
	"log"

	convex "github.com/cervantesh/convex-go"
)

func ExampleClient_authSetup() {
	client, err := convex.NewClient(
		"https://happy-animal-123.convex.cloud",
		convex.WithAuth("jwt-from-your-auth-provider"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	if err := client.SetAuthContext(context.Background(), "rotated-jwt"); err != nil {
		log.Fatal(err)
	}
	if err := client.ClearAuthContext(context.Background()); err != nil {
		log.Fatal(err)
	}
	if err := client.SetAdminAuthContext(context.Background(), "deploy-or-admin-key", convex.UserIdentityAttributes{
		"email": "ada@example.com",
		"name":  "Ada Lovelace",
	}); err != nil {
		log.Fatal(err)
	}
}
