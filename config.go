package main

import (
	"flag"
	"fmt"
	"path"
	"strings"
	"time"
)

// DefaultBuildOutput - default relative path to watch directory to place built binaries in
const DefaultBuildOutput = "bin/app"

// DefaultCommandsDelimiter - default string to split --execs into commands with
const DefaultCommandsDelimiter = ","

// DefaultExecutionGroupsBase - default commands to run when no --execs are specified
var DefaultExecutionGroupsBase = []string{"go mod vendor"}

// DefaultFileExtensions - default commma-separated list of file extensions to watch for
const DefaultFileExtensions = "go,Makefile"

// DefaultIgnoredNames - default comma-separated list of file/dir names to ignore
const DefaultIgnoredNames = "bin,vendor"

// DefaultLogLevel - default log level from 'trace', 'debug', 'info', 'warn', 'error', 'panic'
const DefaultLogLevel = "info"

// DefaultRefreshRate - default duration at which to handle file system events
const DefaultRefreshRate = 2 * time.Second

// Config configures the main application entrypoint
type Config struct {
	RunView           bool
	RunVersion        bool
	RunInit           bool
	RunTest           bool
	LogSilent         bool
	LogVerbose        bool
	LogSuperVerbose   bool
	LogLevel          LogLevel
	FileExtensions    ConfigCommaDelimitedString
	IgnoredNames      ConfigCommaDelimitedString
	ExecGroups        ConfigMultiflagString
	View              string
	CommandsDelimiter string
	BuildOutput       string
	Rate              time.Duration
	WatchDirectory    string
}

// InitConfig creates a configuration from environment variables and flags
func InitConfig() *Config {
	currentWorkingDirectory := getCurrentWorkingDirectory()
	config := &Config{}
	flag.StringVar(&config.View, "view", "", "check out the original content of a file that godev provisions when --init is specified")
	flag.BoolVar(&config.LogSilent, "silent", false, "do not show any logs from godev")
	flag.BoolVar(&config.LogVerbose, "vv", false, "show verbose logs")
	flag.BoolVar(&config.LogSuperVerbose, "vvv", false, "show super verbose logs")
	flag.BoolVar(&config.RunVersion, "version", false, "display the version number")
	flag.BoolVar(&config.RunInit, "init", false, "when this flag is specified, godev initiaises the current directory")
	flag.BoolVar(&config.RunTest, "test", false, "when this flag is specified, godev runs the tests with coverage")
	flag.Var(&config.ExecGroups, "exec", "list of comma-separated commands to run (specify multiple --execs to indicate execution groups)")
	flag.StringVar(&config.CommandsDelimiter, "exec-delim", DefaultCommandsDelimiter, "delimiter character to use to split commands within an execution group")
	flag.Var(&config.FileExtensions, "exts", fmt.Sprintf("comma separated list of file extensions to watch (defaults to: %s)", DefaultFileExtensions))
	flag.Var(&config.IgnoredNames, "ignore", fmt.Sprintf("comma separated list of names to ignore (defaults to: %s)", DefaultIgnoredNames))
	flag.DurationVar(&config.Rate, "rate", DefaultRefreshRate, "specifies the refresh rate of the file system watch")
	flag.StringVar(&config.BuildOutput, "output", DefaultBuildOutput, "specifies the path to the built binary relative to the watch directory (applicable only when --exec is not specified)")
	flag.StringVar(&config.WatchDirectory, "watch", currentWorkingDirectory, "specifies the directory to watch")
	flag.Parse()
	config.assignDefaults()
	config.interpretLogLevel()
	return config
}

func (config *Config) interpretLogLevel() {
	if config.LogVerbose {
		config.LogLevel = "debug"
	}
	if config.LogSuperVerbose {
		config.LogLevel = "trace"
	}
	if config.LogSilent || config.RunVersion {
		config.LogLevel = "panic"
	}
}

func (config *Config) assignDefaults() {
	config.LogLevel = DefaultLogLevel
	config.BuildOutput = path.Join(config.WatchDirectory, "/"+config.BuildOutput)
	config.RunView = len(config.View) > 0
	if len(config.IgnoredNames) == 0 {
		config.IgnoredNames = strings.Split(DefaultIgnoredNames, ",")
	}
	if len(config.FileExtensions) == 0 {
		config.FileExtensions = strings.Split(DefaultFileExtensions, ",")
	}
	if len(config.ExecGroups) == 0 {
		if config.RunTest {
			testFlags := "-coverprofile c.out"
			if config.LogVerbose || config.LogSuperVerbose {
				testFlags = fmt.Sprintf("-v %s", testFlags)
			}
			config.ExecGroups = append(
				DefaultExecutionGroupsBase,
				fmt.Sprintf("go build -o %s", config.BuildOutput),
				fmt.Sprintf("go test ./... %s", testFlags),
			)
		} else {
			config.ExecGroups = append(
				DefaultExecutionGroupsBase,
				fmt.Sprintf("go build -o %s", config.BuildOutput),
				config.BuildOutput,
			)
		}
	}
}