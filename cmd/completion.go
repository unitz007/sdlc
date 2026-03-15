package cmd

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
)

// completionCmd generates shell completion scripts for the sdlc CLI.
var completionCmd = &cobra.Command{
    Use:   "completion",
    Short: "Generate shell completion scripts",
    Long: `Generate shell completion scripts for Bash, Zsh, Fish, and PowerShell.
+
+To load completions:
+
+Bash:
+  $ source <(sdlc completion bash)
+  # To load completions for each session add to your bashrc:
+  $ sdlc completion bash > /etc/bash_completion.d/sdlc
+
+Zsh:
+  $ echo "autoload -U compinit; compinit" >> ~/.zshrc
+  $ sdlc completion zsh > "${fpath[1]}/_sdlc"
+
+Fish:
+  $ sdlc completion fish | source
+  # To load completions for each session add to:
+  $ sdlc completion fish > ~/.config/fish/completions/sdlc.fish
+
+PowerShell:
+  PS> sdlc completion powershell | Out-String | Invoke-Expression
+  # To load completions for every new session:
+  PS> sdlc completion powershell > sdlc.ps1
+`,
    DisableFlagsInUseLine: true,
    Args:                  cobra.ExactValidArgs(1),
    ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
    RunE: func(cmd *cobra.Command, args []string) error {
        switch args[0] {
        case "bash":
            return RootCmd.GenBashCompletion(os.Stdout)
        case "zsh":
            return RootCmd.GenZshCompletion(os.Stdout)
        case "fish":
            return RootCmd.GenFishCompletion(os.Stdout, true)
        case "powershell":
            // GenPowerShellCompletion is available in cobra v1.8.1
            return RootCmd.GenPowerShellCompletion(os.Stdout)
        default:
            return fmt.Errorf("unsupported shell %s", args[0])
        }
    },
}

func init() {
    RootCmd.AddCommand(completionCmd)
}
