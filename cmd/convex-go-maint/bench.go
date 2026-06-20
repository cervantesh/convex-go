package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

type benchmarkMetrics struct {
	NsPerOp     float64
	BytesPerOp  float64
	AllocsPerOp float64
}

func runBenchCompare(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("bench-compare", flag.ContinueOnError)
	flags.SetOutput(stderr)
	oldPath := flags.String("old", "", "path to baseline benchmark output")
	newPath := flags.String("new", "", "path to candidate benchmark output")
	maxRegression := flags.Float64("max-regression", 25, "maximum allowed regression percentage")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*oldPath) == "" || strings.TrimSpace(*newPath) == "" {
		return fmt.Errorf("bench-compare requires -old and -new")
	}
	if *maxRegression < 0 {
		return fmt.Errorf("bench-compare max regression cannot be negative")
	}

	oldBody, err := os.ReadFile(*oldPath)
	if err != nil {
		return fmt.Errorf("read old benchmark output %s: %w", *oldPath, err)
	}
	newBody, err := os.ReadFile(*newPath)
	if err != nil {
		return fmt.Errorf("read new benchmark output %s: %w", *newPath, err)
	}

	report, err := compareBenchmarkOutputs(string(oldBody), string(newBody), *maxRegression)
	if report != "" {
		if _, writeErr := fmt.Fprintln(stdout, report); writeErr != nil {
			return writeErr
		}
	}
	return err
}

func compareBenchmarkOutputs(oldBody, newBody string, maxRegression float64) (string, error) {
	oldBenchmarks, err := parseBenchmarkOutput(oldBody)
	if err != nil {
		return "", err
	}
	newBenchmarks, err := parseBenchmarkOutput(newBody)
	if err != nil {
		return "", err
	}

	names := make([]string, 0, len(oldBenchmarks))
	for name := range oldBenchmarks {
		names = append(names, name)
	}
	sort.Strings(names)

	reportLines := make([]string, 0, len(names))
	failures := make([]string, 0)
	for _, name := range names {
		oldMetrics := oldBenchmarks[name]
		newMetrics, ok := newBenchmarks[name]
		if !ok {
			failures = append(failures, fmt.Sprintf("missing benchmark %s in new output", name))
			continue
		}
		withinBudget := true
		metricReports := make([]string, 0, 3)
		for _, metric := range []struct {
			label string
			old   float64
			new   float64
		}{
			{label: "ns/op", old: oldMetrics.NsPerOp, new: newMetrics.NsPerOp},
			{label: "B/op", old: oldMetrics.BytesPerOp, new: newMetrics.BytesPerOp},
			{label: "allocs/op", old: oldMetrics.AllocsPerOp, new: newMetrics.AllocsPerOp},
		} {
			regression := regressionPercent(metric.old, metric.new)
			metricReports = append(metricReports, fmt.Sprintf("%s %+0.1f%%", metric.label, regression))
			if regression > maxRegression {
				withinBudget = false
				failures = append(failures, fmt.Sprintf("%s %s regression %.1f%% exceeds %.1f%%", name, metric.label, regression, maxRegression))
			}
		}
		status := "within budget"
		if !withinBudget {
			status = "over budget"
		}
		reportLines = append(reportLines, fmt.Sprintf("%s: %s (%s)", name, strings.Join(metricReports, ", "), status))
	}
	for name := range newBenchmarks {
		if _, ok := oldBenchmarks[name]; !ok {
			reportLines = append(reportLines, fmt.Sprintf("%s: new benchmark in candidate output", name))
		}
	}
	if len(failures) > 0 {
		return strings.Join(reportLines, "\n"), errors.New(strings.Join(failures, "; "))
	}
	return strings.Join(reportLines, "\n"), nil
}

func parseBenchmarkOutput(body string) (map[string]benchmarkMetrics, error) {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	benchmarks := make(map[string]benchmarkMetrics)
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "Benchmark") {
			continue
		}
		name, metrics, err := parseBenchmarkLine(line)
		if err != nil {
			return nil, err
		}
		benchmarks[name] = metrics
	}
	if len(benchmarks) == 0 {
		return nil, errors.New("no benchmark lines found")
	}
	return benchmarks, nil
}

func parseBenchmarkLine(line string) (string, benchmarkMetrics, error) {
	fields := strings.Fields(line)
	if len(fields) < 8 {
		return "", benchmarkMetrics{}, fmt.Errorf("invalid benchmark line %q", line)
	}
	name := normalizeBenchmarkName(fields[0])
	var metrics benchmarkMetrics
	var sawNS, sawBytes, sawAllocs bool
	for i := 2; i+1 < len(fields); i += 2 {
		value, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return "", benchmarkMetrics{}, fmt.Errorf("parse benchmark metric %q in %q: %w", fields[i], line, err)
		}
		switch fields[i+1] {
		case "ns/op":
			metrics.NsPerOp = value
			sawNS = true
		case "B/op":
			metrics.BytesPerOp = value
			sawBytes = true
		case "allocs/op":
			metrics.AllocsPerOp = value
			sawAllocs = true
		}
	}
	if !sawNS || !sawBytes || !sawAllocs {
		return "", benchmarkMetrics{}, fmt.Errorf("benchmark line %q must include ns/op, B/op, and allocs/op", line)
	}
	return name, metrics, nil
}

func normalizeBenchmarkName(name string) string {
	dash := strings.LastIndex(name, "-")
	if dash == -1 || dash == len(name)-1 {
		return name
	}
	for _, r := range name[dash+1:] {
		if r < '0' || r > '9' {
			return name
		}
	}
	return name[:dash]
}

func regressionPercent(oldValue, newValue float64) float64 {
	if oldValue == 0 {
		if newValue == 0 {
			return 0
		}
		return 100
	}
	return ((newValue - oldValue) * 100) / oldValue
}
