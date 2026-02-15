package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kalt/liviva/internal/client"
	"github.com/kalt/liviva/internal/server"
)

func main() {
	// Root Command (No args -> Help)
	var rootCmd = &cobra.Command{
		Use:   "liviva",
		Short: "LIVIVA: Local Intelligent Virtual Intelligence & Versatile Assistant",
		Long: `LIVIVA is an AI assistant that runs locally on Linux infrastructure,
using external LLMs for reasoning. It operates as a client-server system.`,
	}

	// Server Command (Runs the Agent)
	var serverCmd = &cobra.Command{
		Use:   "server",
		Short: "Starts the LIVIVA Agent Server (Daemon)",
		Run: func(cmd *cobra.Command, args []string) {
			server.Run()
		},
	}

	// Client Command (Interactive Terminal)
	var clientCmd = &cobra.Command{
		Use:   "connect [address]",
		Short: "Connects to a running LIVIVA Server",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			addr := "localhost:50051" // Default
			if len(args) > 0 {
				addr = args[0]
			}
			client.Run(addr)
		},
	}

	// Add commands
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(clientCmd)

	// Execute
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
