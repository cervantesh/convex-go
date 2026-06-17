package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/cervantesh/convex-go/internal/codegen"
)

const defaultImportPath = "github.com/cervantesh/convex-go"

func main() {
	if err := runWithIO(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		os.Exit(1)
	}
}

func run(args []string) error {
	return runWithIO(args, os.Stdout, os.Stderr)
}

func runWithIO(args []string, stdout io.Writer, stderr io.Writer) error {
	var config codegen.Config
	var outPath string
	var dryRun bool
	var check bool
	flags := flag.NewFlagSet("convex-go-codegen", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&config.SourceDir, "convex-dir", "convex", "path to the Convex functions directory")
	flags.StringVar(&config.SourceDir, "src", "convex", "path to the Convex functions directory")
	flags.StringVar(&config.PackageName, "package", "convexapi", "generated Go package name")
	flags.StringVar(&config.ImportPath, "import", defaultImportPath, "convex-go module import path")
	flags.StringVar(&config.ImportPath, "convex-import", defaultImportPath, "convex-go module import path")
	flags.StringVar(&outPath, "out", "", "output Go file path")
	flags.BoolVar(&dryRun, "dry-run", false, "write generated Go to stdout instead of a file")
	flags.BoolVar(&check, "check", false, "fail if the output file differs from generated content")
	if err := flags.Parse(args); err != nil {
		return err
	}
	generated, err := codegen.Generate(config)
	if err != nil {
		reportError(stderr, err)
		return err
	}
	if dryRun {
		_, err := stdout.Write(generated)
		return err
	}
	if outPath == "" {
		err := fmt.Errorf("convex-go-codegen: -out is required")
		reportError(stderr, err)
		return err
	}
	if check {
		current, err := os.ReadFile(outPath)
		if err != nil {
			reportError(stderr, err)
			return err
		}
		if !bytes.Equal(current, generated) {
			err := fmt.Errorf("convex-go-codegen: %s is not up to date", outPath)
			reportError(stderr, err)
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		reportError(stderr, err)
		return err
	}
	if err := os.WriteFile(outPath, generated, 0o644); err != nil {
		reportError(stderr, err)
		return err
	}
	return nil
}

func reportError(w io.Writer, err error) {
	_, _ = fmt.Fprintln(w, err)
}
