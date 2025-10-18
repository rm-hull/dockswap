package main

import (
	"log"

	"github.com/rm-hull/dockswap/cmd"
	"github.com/spf13/cobra"
)

func main() {

	var (
		serviceName    string
		projectName    string
		imageName      string
		waitSeconds    int
		timeoutSeconds int
	)

	var rootCmd = &cobra.Command{
		Use:   "dockswap",
		Short: "Perform safe, health-checked rolling restarts of Docker Compose services",
		Long: `dockwap pulls the latest image, starts a new container, waits until it passes its HEALTHCHECK, then stops the old one — ensuring zero downtime.

Features:
-   Sequential rolling updates for Docker Compose services
-   Uses container healthchecks to ensure readiness
-   Works with multiple replicas
-   Pure Docker Engine API (no shell commands)`,
		Run: func(_ *cobra.Command, args []string) {
			if err := cmd.Run(serviceName, projectName, imageName, timeoutSeconds, waitSeconds); err != nil {
				log.Fatal(err)
			}
		},
	}

	rootCmd.Flags().StringVarP(&serviceName, "service", "s", "", "Docker Compose service name (required)")
	rootCmd.Flags().StringVarP(&projectName, "project", "p", "", "Docker Compose project name (required)")
	rootCmd.Flags().StringVarP(&imageName, "image", "i", "", "Image to pull and deploy (required)")
	rootCmd.Flags().IntVarP(&waitSeconds, "wait", "w", 5, "Seconds between health checks")
	rootCmd.Flags().IntVarP(&timeoutSeconds, "timeout", "t", 120, "Max seconds to wait for container to be healthy")

	rootCmd.MarkFlagRequired("service")
	rootCmd.MarkFlagRequired("project")
	rootCmd.MarkFlagRequired("image")

	rootCmd.Execute()
}
