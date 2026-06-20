package baseclient

import "testing"

func BenchmarkReplayOngoingRequests(b *testing.B) {
	client := New()
	if _, err := client.Subscribe("messages:list", nil); err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 16; i++ {
		if _, err := client.Mutation("messages:send", map[string]any{"body": "bench"}); err != nil {
			b.Fatal(err)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		replayed := client.ReplayOngoingRequests()
		if len(replayed) != 16 {
			b.Fatalf("replay count = %d, want 16", len(replayed))
		}
	}
}
