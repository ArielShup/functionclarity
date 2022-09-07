package aws

import (
	"fmt"
	opt "github.com/openclarity/function-clarity/cmd/function-clarity/cli/options"
	"github.com/openclarity/function-clarity/pkg/clients"
	i "github.com/openclarity/function-clarity/pkg/init"
	"github.com/openclarity/function-clarity/pkg/options"
	"github.com/openclarity/function-clarity/pkg/verify"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
	"log"
	"os"
)

func AwsSign() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aws",
		Short: "sign content from aws",
	}
	cmd.AddCommand(AwsSignCode())
	cmd.AddCommand(AwsSignImage())
	return cmd
}

func AwsVerify() *cobra.Command {
	o := &options.VerifyOpts{}
	var functionIdentifier string
	cmd := &cobra.Command{
		Use:   "aws",
		Short: "verify function identity",
		Args:  cobra.ExactArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			if err := viper.BindPFlag("accessKey", cmd.Flags().Lookup("aws-access-key")); err != nil {
				log.Fatal(err)
			}
			if err := viper.BindPFlag("secretKey", cmd.Flags().Lookup("aws-secret-key")); err != nil {
				log.Fatal(err)
			}
			if err := viper.BindPFlag("region", cmd.Flags().Lookup("region")); err != nil {
				log.Fatal(err)
			}
			if err := viper.BindPFlag("bucket", cmd.Flags().Lookup("bucket")); err != nil {
				log.Fatal(err)
			}
			if err := viper.BindPFlag("publickey", cmd.Flags().Lookup("key")); err != nil {
				log.Fatal(err)
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			awsClient := clients.NewAwsClient(viper.GetString("accesskey"), viper.GetString("secretkey"), viper.GetString("bucket"), viper.GetString("region"))
			return verify.Verify(awsClient, args[0], o, cmd.Context())
		},
	}
	cmd.Flags().StringVar(&functionIdentifier, "function-identifier", "",
		"function to verify")
	o.AddFlags(cmd)
	initAwsVerifyFlags(cmd)
	return cmd
}

func initAwsVerifyFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&opt.Config, "config", "", "config file (default: $HOME/.fs)")
	cmd.Flags().String("aws-access-key", "", "aws access key")
	cmd.Flags().String("aws-secret-key", "", "aws secret key")
	cmd.Flags().String("region", "", "aws region to perform the operation against")
	cmd.Flags().String("bucket", "", "s3 bucket to work against")
	cmd.Flags().String("key", "", "public key")
}

func AwsInit() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aws",
		Short: "initialize configuration in aws",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var input i.AWSInput
			if err := input.ReceiveParameters(); err != nil {
				return err
			}
			awsClient := clients.NewAwsClientInit(input.AccessKey, input.SecretKey, input.Region)
			err := awsClient.DeployFunctionClarity(input.CloudTrail.Name, input.PublicKey)
			if err != nil {
				return err
			}
			d, err := yaml.Marshal(&input)
			if err != nil {
				log.Fatalf("error converting init configuration to YAML: %v", err)
			}

			h, err := os.UserHomeDir()
			if err != nil {
				log.Fatal(err)
			}
			f, err := os.Create(h + "/.fc")
			if err != nil {
				return fmt.Errorf("failed to create configuration file: %v", err)
			}
			defer f.Close()
			if _, err = f.Write(d); err != nil {
				return fmt.Errorf("failed to write configuration to file: %v", err)
			}
			return nil
		},
	}
	return cmd
}
