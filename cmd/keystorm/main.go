// Package main is the entry point for the Keystorm editor.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/dshills/keystorm/internal/app"
	"github.com/dshills/keystorm/internal/renderer/backend"
)

// Version information (set via ldflags during build).
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	os.Exit(run())
}

func run() int {
	opts := parseFlags()

	// Create application
	application, err := app.New(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to initialize: %v\n", err)
		return 1
	}

	// Ensure cleanup on all exit paths
	defer application.Shutdown()

	// Create terminal backend
	term, err := backend.NewTerminal()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create terminal: %v\n", err)
		return 1
	}
	if err := application.SetBackend(term); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to set backend: %v\n", err)
		return 1
	}

	// Handle signals for graceful shutdown
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-signals
		application.Shutdown()
	}()

	// Run the application
	if err := application.Run(); err != nil {
		// Check if it's a normal quit using errors.Is for wrapped errors
		if errors.Is(err, app.ErrQuit) {
			return 0
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	return 0
}

func parseFlags() app.Options {
	var opts app.Options
	var showVersion bool
	var showHelp bool

	flag.StringVar(&opts.ConfigPath, "config", "", "Path to configuration file")
	flag.StringVar(&opts.ConfigPath, "c", "", "Path to configuration file (shorthand)")
	flag.StringVar(&opts.WorkspacePath, "workspace", "", "Workspace/project directory")
	flag.StringVar(&opts.WorkspacePath, "w", "", "Workspace/project directory (shorthand)")
	flag.BoolVar(&opts.Debug, "debug", false, "Enable debug mode")
	flag.BoolVar(&opts.Debug, "d", false, "Enable debug mode (shorthand)")
	flag.StringVar(&opts.LogLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	flag.BoolVar(&opts.ReadOnly, "readonly", false, "Open files in read-only mode")
	flag.BoolVar(&opts.ReadOnly, "R", false, "Open files in read-only mode (shorthand)")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.BoolVar(&showVersion, "v", false, "Show version information (shorthand)")
	flag.BoolVar(&showHelp, "help", false, "Show help message")
	flag.BoolVar(&showHelp, "h", false, "Show help message (shorthand)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Keystorm - AI-native programming editor\n\n")
		fmt.Fprintf(os.Stderr, "Usage: keystorm [options] [files...]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  keystorm                    Open with empty buffer\n")
		fmt.Fprintf(os.Stderr, "  keystorm file.go            Open a file\n")
		fmt.Fprintf(os.Stderr, "  keystorm -w ./project       Open workspace\n")
		fmt.Fprintf(os.Stderr, "  keystorm -R file.go         Open file read-only\n")
	}

	flag.Parse()

	if showHelp {
		flag.Usage()
		os.Exit(0)
	}

	if showVersion {
		fmt.Printf("Keystorm %s\n", version)
		fmt.Printf("Commit: %s\n", commit)
		fmt.Printf("Built: %s\n", date)
		os.Exit(0)
	}

	// Validate log level
	switch opts.LogLevel {
	case "debug", "info", "warn", "error":
		// Valid
	default:
		fmt.Fprintf(os.Stderr, "Error: invalid log level %q (must be debug, info, warn, or error)\n", opts.LogLevel)
		os.Exit(1)
	}

	// Remaining arguments are files to open
	opts.Files = flag.Args()

	// If workspace not specified but files are, use the directory of the first file
	if opts.WorkspacePath == "" && len(opts.Files) > 0 {
		absPath, err := filepath.Abs(opts.Files[0])
		if err == nil {
			opts.WorkspacePath = filepath.Dir(absPath)
		}
	}

	return opts
}
