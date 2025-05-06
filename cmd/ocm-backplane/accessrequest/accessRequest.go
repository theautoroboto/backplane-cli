package accessrequest

import (
	"fmt"
	"os"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewAccessRequestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "accessrequest",
		Aliases:      []string{"accessRequest", "accessrequest", "accessrequests"},
		Short:        "Manages access requests for clusters on which access protection is enabled",
		SilenceUsage: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Check if the --govcloud flag is set
			if viper.GetBool("govcloud") {
				fmt.Println("Skipping accessrequest command because --govcloud is set")
				// Skip the rest of the command execution
				cmd.Help() // Optionally show help or a message
				os.Exit(0) // Exit the program to skip further execution
			}
		},
	}

	// cluster-id Flag
	cmd.PersistentFlags().StringP("cluster-id", "c", "", "Cluster ID could be cluster name, id or external-id")

	cmd.AddCommand(newCreateAccessRequestCmd())
	cmd.AddCommand(newGetAccessRequestCmd())
	cmd.AddCommand(newExpireAccessRequestCmd())

	return cmd
}

func init() {
}
