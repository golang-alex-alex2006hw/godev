//go:generate go run data/generate.go
package main

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"

	shellquote "github.com/kballard/go-shellquote"
)

func main() {
	app := initCLI()
	app.Start(os.Args, func(config *Config) {
		godev := InitGoDev(config)
		godev.Start()
	})
}

// InitGoDev initialises the application using a configuration
// struct and creating a logger
func InitGoDev(config *Config) *GoDev {
	return &GoDev{
		config: config,
		logger: InitLogger(&LoggerConfig{
			Name:   "main",
			Format: "production",
			Level:  config.LogLevel,
		}),
	}
}

// GoDev holds the logic and values needed for GoDev to run
type GoDev struct {
	config  *Config
	logger  *Logger
	watcher *Watcher
	runner  *Runner
}

// Start should only be called once and triggers the pipeline
// and watcher
func (godev *GoDev) Start() {
	defer godev.logger.Infof("godev has ended")
	godev.logger.Infof("godev has started")
	if godev.config.RunDefault || godev.config.RunTest {
		godev.startWatching()
	} else if godev.config.RunInit {
		godev.initialiseDirectory()
	}
}

func (godev *GoDev) createPipeline() []*ExecutionGroup {
	var pipeline []*ExecutionGroup
	for execGroupIndex, execGroup := range godev.config.ExecGroups {
		executionGroup := &ExecutionGroup{}
		var executionCommands []*Command
		commands := strings.Split(execGroup, godev.config.CommandsDelimiter)
		for _, command := range commands {
			if sections, err := shellquote.Split(command); err != nil {
				panic(err)
			} else {
				arguments := sections[1:]
				if execGroupIndex == len(godev.config.ExecGroups)-1 {
					arguments = append(arguments, godev.config.CommandArguments...)
				}
				executionCommands = append(
					executionCommands,
					InitCommand(&CommandConfig{
						Application: sections[0],
						Arguments:   arguments,
						Directory:   godev.config.WorkDirectory,
						Environment: godev.config.EnvVars,
						LogLevel:    godev.config.LogLevel,
					}),
				)
			}
		}
		executionGroup.commands = executionCommands
		pipeline = append(pipeline, executionGroup)
	}
	return pipeline
}

func (godev *GoDev) eventHandler(events *[]WatcherEvent) bool {
	for _, e := range *events {
		godev.logger.Trace(e)
	}
	godev.runner.Trigger()
	return true
}

func (godev *GoDev) initialiseInitialisers() []Initialiser {
	return []Initialiser{
		InitGitInitialiser(&GitInitialiserConfig{
			Path: path.Join(godev.config.WorkDirectory),
		}),
		InitFileInitialiser(&FileInitialiserConfig{
			Path:     path.Join(godev.config.WorkDirectory, "/.gitignore"),
			Data:     []byte(DataDotGitignore),
			Question: "seed a .gitignore?",
		}),
		InitFileInitialiser(&FileInitialiserConfig{
			Path:     path.Join(godev.config.WorkDirectory, "/go.mod"),
			Data:     []byte(DataGoDotMod),
			Question: "seed a go.mod?",
		}),
		InitFileInitialiser(&FileInitialiserConfig{
			Path:     path.Join(godev.config.WorkDirectory, "/main.go"),
			Data:     []byte(DataMainDotgo),
			Question: "seed a main.go?",
		}),
		InitFileInitialiser(&FileInitialiserConfig{
			Path:     path.Join(godev.config.WorkDirectory, "/Dockerfile"),
			Data:     []byte(DataDockerfile),
			Question: "seed a Dockerfile?",
		}),
		InitFileInitialiser(&FileInitialiserConfig{
			Path:     path.Join(godev.config.WorkDirectory, "/.dockerignore"),
			Data:     []byte(DataDotDockerignore),
			Question: "seed a .dockerignore?",
		}),
		InitFileInitialiser(&FileInitialiserConfig{
			Path:     path.Join(godev.config.WorkDirectory, "/Makefile"),
			Data:     []byte(DataMakefile),
			Question: "seed a Makefile?",
		}),
	}
}

// initialiseDirectory assists in initialising the working directory
func (godev *GoDev) initialiseDirectory() {
	if !directoryExists(godev.config.WorkDirectory) {
		godev.logger.Errorf("the directory at '%s' does not exist - create it first with:\n  mkdir -p %s", godev.config.WorkDirectory, godev.config.WorkDirectory)
		os.Exit(1)
	}
	initialisers := godev.initialiseInitialisers()
	for i := 0; i < len(initialisers); i++ {
		initialiser := initialisers[i]
		if initialiser.Check() {
			err := initialiser.Handle(true)
			if err != nil {
				fmt.Println(Color("red", err.Error()))
			}
		} else {
			reader := bufio.NewReader(os.Stdin)
			if initialiser.Confirm(reader) {
				fmt.Println(Color("green", "godev> sure thing"))
				initialiser.Handle()
			} else {
				fmt.Println(Color("yellow", "godev> lets skip that then"))
			}
		}
	}
}

func (godev *GoDev) initialiseRunner() {
	godev.runner = InitRunner(&RunnerConfig{
		Pipeline: godev.createPipeline(),
		LogLevel: godev.config.LogLevel,
	})
}

func (godev *GoDev) initialiseWatcher() {
	godev.watcher = InitWatcher(&WatcherConfig{
		FileExtensions: godev.config.FileExtensions,
		IgnoredNames:   godev.config.IgnoredNames,
		RefreshRate:    godev.config.Rate,
		LogLevel:       godev.config.LogLevel,
	})
	godev.watcher.RecursivelyWatch(godev.config.WatchDirectory)
}

func (godev *GoDev) logUniversalConfigurations() {
	godev.logger.Debugf("flag - init       : %v", godev.config.RunInit)
	godev.logger.Debugf("flag - test       : %v", godev.config.RunTest)
	godev.logger.Debugf("flag - view       : %v", godev.config.RunView)
	godev.logger.Debugf("watch directory   : %s", godev.config.WatchDirectory)
	godev.logger.Debugf("work directory    : %s", godev.config.WorkDirectory)
	godev.logger.Debugf("build output      : %s", godev.config.BuildOutput)
}

func (godev *GoDev) logWatchModeConfigurations() {
	config := godev.config
	logger := godev.logger
	logger.Debugf("environment       : %v", config.EnvVars)
	logger.Debugf("file extensions   : %v", config.FileExtensions)
	logger.Debugf("ignored names     : %v", config.IgnoredNames)
	logger.Debugf("refresh interval  : %v", config.Rate)
	logger.Debugf("execution delim   : %s", config.CommandsDelimiter)
	logger.Debug("execution groups as follows...")
	for execGroupIndex, execGroup := range config.ExecGroups {
		logger.Debugf("  %v) %s", execGroupIndex+1, execGroup)
		commands := strings.Split(execGroup, config.CommandsDelimiter)
		for commandIndex, command := range commands {
			sections, err := shellquote.Split(command)
			if err != nil {
				panic(err)
			}
			application := sections[0]
			arguments := sections[1:]
			if execGroupIndex == len(config.ExecGroups)-1 {
				arguments = append(arguments, config.CommandArguments...)
			}
			logger.Debugf("    %v > %s %v", commandIndex+1, application, arguments)
		}
	}
}

func (godev *GoDev) startWatching() {
	godev.logUniversalConfigurations()
	godev.logWatchModeConfigurations()
	godev.initialiseWatcher()
	godev.initialiseRunner()

	var wg sync.WaitGroup
	godev.watcher.BeginWatch(&wg, godev.eventHandler)
	godev.logger.Infof("working dir : '%s'", godev.config.WorkDirectory)
	godev.logger.Infof("watching dir: '%s'", godev.config.WatchDirectory)
	godev.runner.Trigger()
	wg.Wait()
}
