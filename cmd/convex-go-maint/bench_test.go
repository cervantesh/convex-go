package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseBenchmarkOutputExtractsMetrics(t *testing.T) {
	body := strings.TrimSpace(`
 goos: linux
 pkg: example.com/convex-go
 BenchmarkValueJSONRoundTrip-8             1000           1200 ns/op          320 B/op         12 allocs/op
 BenchmarkReplayOngoingRequests-8         50000             90 ns/op           48 B/op          3 allocs/op
 PASS
 `)
	benchmarks, err := parseBenchmarkOutput(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(benchmarks) != 2 {
		t.Fatalf("benchmark count = %d, want 2", len(benchmarks))
	}
	value, ok := benchmarks["BenchmarkValueJSONRoundTrip"]
	if !ok {
		t.Fatal("expected BenchmarkValueJSONRoundTrip entry")
	}
	if value.NsPerOp != 1200 || value.BytesPerOp != 320 || value.AllocsPerOp != 12 {
		t.Fatalf("unexpected value benchmark metrics: %#v", value)
	}
}

func TestCompareBenchmarkOutputsRejectsRegression(t *testing.T) {
	oldBody := "BenchmarkValueJSONRoundTrip-8 1000 100 ns/op 32 B/op 4 allocs/op\n"
	newBody := "BenchmarkValueJSONRoundTrip-8 1000 140 ns/op 32 B/op 4 allocs/op\n"

	_, err := compareBenchmarkOutputs(oldBody, newBody, 25)
	if err == nil || !strings.Contains(err.Error(), "ns/op regression 40.0%") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWithIOBenchCompareSuccess(t *testing.T) {
	root := t.TempDir()
	oldPath := filepath.Join(root, "old.txt")
	newPath := filepath.Join(root, "new.txt")
	if err := os.WriteFile(oldPath, []byte("BenchmarkReplayOngoingRequests-8 1000 100 ns/op 32 B/op 4 allocs/op\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("BenchmarkReplayOngoingRequests-8 1000 120 ns/op 32 B/op 4 allocs/op\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	if err := runWithIO([]string{"bench-compare", "-old", oldPath, "-new", newPath, "-max-regression", "25"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"BenchmarkReplayOngoingRequests",
		"ns/op +20.0%",
		"within budget",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout must contain %q, got %q", want, stdout.String())
		}
	}
}

func TestRunWithIOBenchCompareMissingBenchmarkFails(t *testing.T) {
	root := t.TempDir()
	oldPath := filepath.Join(root, "old.txt")
	newPath := filepath.Join(root, "new.txt")
	if err := os.WriteFile(oldPath, []byte("BenchmarkValueJSONRoundTrip-8 1000 100 ns/op 32 B/op 4 allocs/op\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("BenchmarkReplayOngoingRequests-8 1000 100 ns/op 32 B/op 4 allocs/op\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runWithIO([]string{"bench-compare", "-old", oldPath, "-new", newPath}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "missing benchmark") {
		t.Fatalf("unexpected error: %v", err)
	}
}
