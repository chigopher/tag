package cmd

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var logger zerolog.Logger

type timestampHook struct{}

func (t timestampHook) Run(e *zerolog.Event, level zerolog.Level, message string) {
	e.Timestamp()
}

func maybeExit(err error) {
	fmt.Printf("%v\n", err)
	if err != nil {
		os.Exit(1)
	}
}

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "mockery_tools [command]",
	}

	logger = zerolog.New(zerolog.ConsoleWriter{
		Out: os.Stderr,
	}).Hook(timestampHook{})

	subCommands := []func() (*cobra.Command, error){
		NewTagCmd,
	}
	for _, CommandFunc := range subCommands {
		subCmd, err := CommandFunc()
		if err != nil {
			panic(err)
		}
		cmd.AddCommand(subCmd)
	}
	return cmd
}

func handleErr(err error) {
	fmt.Printf("%v\n", err)
	os.Exit(1)
}
