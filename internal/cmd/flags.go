// Package cmd provides CLI flag registration using a modular pattern.
//
// To add a new flag without causing merge conflicts:
// 1. Create a new file (e.g., flags_myfeature.go)
// 2. Define your flag variable and default at the top
// 3. Create an init() function that calls RegisterFlag()
//
// Example (flags_myfeature.go):
//
//	package cmd
//
//	var myFeatureEnabled bool
//
//	func init() {
//		RegisterFlag(func(cmd *cobra.Command) {
//			cmd.Flags().BoolVar(&myFeatureEnabled, "my-feature", false, "Enable my feature")
//		})
//	}
package cmd

import "github.com/spf13/cobra"

// FlagRegistrar is a function that registers flags on a command
type FlagRegistrar func(cmd *cobra.Command)

// flagRegistrars holds all flag registration functions
var flagRegistrars []FlagRegistrar

// RegisterFlag adds a flag registrar to be called during init
// This allows each feature to register its own flags without modifying root.go
func RegisterFlag(fn FlagRegistrar) {
	flagRegistrars = append(flagRegistrars, fn)
}

// registerAllFlags calls all registered flag registrars
func registerAllFlags(cmd *cobra.Command) {
	for _, fn := range flagRegistrars {
		fn(cmd)
	}
}
