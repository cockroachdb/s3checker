/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"context"
	"github.com/cockroachdb/s3checker/s3checker"
	"github.com/spf13/viper"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "s3checker",
	Version: "0.1.0",
	Short:   "Check for S3 bucket access and privileges",
	Long: `s3checker checks for S3 bucket access and privileges required by CockroachDB cloud operations.
It uses the same S3 library that CockroachDB uses.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		bucket := viper.GetString("bucket")
		auth := viper.GetString("auth")
		keyId := viper.GetString("key-id")
		accessKey := viper.GetString("access-key")
		sessionToken := viper.GetString("session-token")
		region := viper.GetString("region")
		debug := viper.GetBool("debug")
		version := viper.GetInt("sdk-version")

		ctx := context.Background()
		if version == 2 {
			err := s3checker.CheckV2(ctx, bucket, auth, keyId, accessKey, sessionToken, region, debug)
			if err != nil {
				panic(err)
			}
		} else {
			err := s3checker.Check(bucket, auth, keyId, accessKey, sessionToken, region, debug)
			if err != nil {
				panic(err)
			}
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.s3checker.yaml)")
	rootCmd.PersistentFlags().String("auth", "implicit", "Auth type: implicit or explicit")
	rootCmd.PersistentFlags().String("bucket", "", "S3 bucket")
	rootCmd.PersistentFlags().String("key-id", "", "AWS access key ID, when using explicit auth")
	rootCmd.PersistentFlags().String("access-key", "", "AWS secret access key, when using explicit auth")
	rootCmd.PersistentFlags().String("session-token", "", "AWS session token, when using explicit auth and STS temporary credentials")
	rootCmd.PersistentFlags().String("region", "", "AWS region, optional")
	rootCmd.PersistentFlags().Bool("debug", false, "Include debug output for request errors")
	rootCmd.PersistentFlags().Int("sdk-version", 1, "AWS SDK version, 1 or 2 (default 1)")

	rootCmd.MarkPersistentFlagRequired("bucket")
	rootCmd.MarkFlagsRequiredTogether("key-id", "access-key")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	viper.BindPFlags(rootCmd.PersistentFlags())
}
