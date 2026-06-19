package convex

import "github.com/cervantesh/convex-go/baseclient"

// UserTokenFetcher returns a user JWT for HTTP and realtime requests.
// When forceRefresh is true, the fetcher should bypass any cached token.
type UserTokenFetcher func(forceRefresh bool) (string, error)

func adaptUserTokenFetcher(fetcher UserTokenFetcher) baseclient.AuthTokenFetcher {
	if fetcher == nil {
		return nil
	}
	return func(forceRefresh bool) (baseclient.AuthToken, error) {
		token, err := fetcher(forceRefresh)
		if err != nil {
			return baseclient.AuthToken{}, err
		}
		if token == "" {
			return baseclient.NoAuthToken(), nil
		}
		return baseclient.UserAuthToken(token), nil
	}
}
