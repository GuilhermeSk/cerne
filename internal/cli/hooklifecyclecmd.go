package cli

import "github.com/spf13/cobra"

// Retired lifecycle reminder (F-0170 → F-0426). The PostToolUse hook used to
// inject the moment-of-action guidance after `feature new`, `idea promote` and
// `feature start`; that guidance now rides the commands' own stdout (the Next
// step blocks in transitioncmd.go), the one channel every harness delivers.
// `cerne hook install` no longer wires it. The subcommand stays, silent and
// exiting 0, because projects installed before F-0426 still call it from
// their settings — emitting anything here would hand Claude Code the guidance
// twice.
func newHookLifecycleReminderCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "lifecycle-reminder",
		Short:  "Retired PostToolUse plumbing (F-0426): silent; the CLI prints the guidance",
		Hidden: true,
		RunE:   func(*cobra.Command, []string) error { return nil },
	}
}
