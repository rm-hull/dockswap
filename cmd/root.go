package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/spf13/cobra"
)

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
	Run: func(cmd *cobra.Command, args []string) {
		if err := run(); err != nil {
			log.Fatal(err)
		}
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&serviceName, "service", "s", "", "Docker Compose service name (required)")
	rootCmd.Flags().StringVarP(&projectName, "project", "p", "", "Docker Compose project name (required)")
	rootCmd.Flags().StringVarP(&imageName, "image", "i", "", "Image to pull and deploy (required)")
	rootCmd.Flags().IntVarP(&waitSeconds, "wait", "w", 5, "Seconds between health checks")
	rootCmd.Flags().IntVarP(&timeoutSeconds, "timeout", "t", 120, "Max seconds to wait for container to be healthy")

	rootCmd.MarkFlagRequired("service")
	rootCmd.MarkFlagRequired("project")
	rootCmd.MarkFlagRequired("image")
}

func run() error {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	// 1. Find running containers for this Compose service
	f := filters.NewArgs()
	f.Add("label", fmt.Sprintf("com.docker.compose.service=%s", serviceName))
	f.Add("label", fmt.Sprintf("com.docker.compose.project=%s", projectName))
	containers, err := cli.ContainerList(ctx, container.ListOptions{Filters: f})
	if err != nil {
		return err
	}
	if len(containers) == 0 {
		return fmt.Errorf("no running containers found for service %s in project %s", serviceName, projectName)
	}

	log.Printf("Found %d containers for service %s.\n", len(containers), serviceName)

	// 2. Pull latest image
	log.Printf("Pulling latest image: %s\n", imageName)
	reader, err := cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return err
	}
	if _, err := io.Copy(os.Stdout, reader); err != nil {
		return fmt.Errorf("failed to copy to stdout: %v", err)
	}

	// 3. Replace each container one-by-one
	for _, old := range containers {
		if err := replaceContainer(ctx, cli, old, timeoutSeconds, waitSeconds); err != nil {
			return fmt.Errorf("failed to replace container %s: %w", old.ID[:12], err)
		}
	}

	log.Println("\nRolling restart completed successfully.")
	return nil
}

func replaceContainer(ctx context.Context, cli *client.Client, old container.Summary, timeoutSeconds, waitSeconds int) error {
	log.Printf("\n--- Replacing container %s (%s) ---", old.ID[:12], strings.Join(old.Names, ", "))

	// Get old container config
	inspect, err := cli.ContainerInspect(ctx, old.ID)
	if err != nil {
		return err
	}

	// Create new container
	resp, err := cli.ContainerCreate(
		ctx,
		inspect.Config,
		inspect.HostConfig,
		&network.NetworkingConfig{
			EndpointsConfig: inspect.NetworkSettings.Networks,
		},
		nil,
		"",
	)
	if err != nil {
		return err
	}

	// Start new container
	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return err
	}
	log.Printf("Started new container: %s\n", resp.ID[:12])

	// Wait for healthy
	log.Println("Waiting for new container to be healthy...")
	healthyCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	if err := waitForHealthy(healthyCtx, cli, resp.ID, time.Duration(waitSeconds)*time.Second); err != nil {
		return fmt.Errorf("new container %s did not become healthy: %v", resp.ID[:12], err)
	}
	log.Printf("✅ New container %s is healthy!\n", resp.ID[:12])

	// Stop & remove old container
	log.Printf("Stopping old container: %s\n", old.ID[:12])
	cli.ContainerStop(ctx, old.ID, container.StopOptions{})
	cli.ContainerRemove(ctx, old.ID, container.RemoveOptions{})
	log.Println("Old container removed.")
	return nil
}

func waitForHealthy(ctx context.Context, cli *client.Client, containerID string, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			inspect, err := cli.ContainerInspect(ctx, containerID)
			if err != nil {
				return err
			}
			if inspect.State != nil {
				if inspect.State.Health != nil {
					if inspect.State.Health.Status == "healthy" {
						return nil
					}
				} else if inspect.State.Status == "running" {
					// No health check configured, consider "running" as healthy
					return nil
				}
			}
		}
	}
}
