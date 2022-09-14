package cli

import (
	"github.com/openclarity/function-clarity/cmd/function-clarity/cli/aws"
	"github.com/spf13/cobra"
)

type SignOptions struct {
	key string
}

func Sign() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sign",
		Short: "sign image/folder and upload the signature to cloud provider",
	}
	cmd.AddCommand(aws.AwsSign())
	return cmd
}
