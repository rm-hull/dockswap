package main

import (
	"context"
	"flag"
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
)

func main() {
	// CLI flags
	serviceName := flag.String("service", "", "Docker Compose service name (required)")
	projectName := flag.String("project", "", "Docker Compose project name (required)")
	imageName := flag.String("image", "", "Image to pull and deploy (required)")
	waitSeconds := flag.Int("wait", 5, "Seconds between health checks")
	timeoutSeconds := flag.Int("timeout", 120, "Max seconds to wait for container to be healthy")
	flag.Parse()

	if *serviceName == "" || *projectName == "" || *imageName == "" {
		flag.Usage()
		os.Exit(1)
	}

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(err)
	}

	// 1. Find running containers for this Compose service
	f := filters.NewArgs()
	f.Add("label", fmt.Sprintf("com.docker.compose.service=%s", *serviceName))
	f.Add("label", fmt.Sprintf("com.docker.compose.project=%s", *projectName))
	containers, err := cli.ContainerList(ctx, container.ListOptions{Filters: f})
	if err != nil {
		log.Fatal(err)
	}
	if len(containers) == 0 {
		log.Fatalf("No running containers found for service %s in project %s", *serviceName, *projectName)
	}

	fmt.Printf("Found %d containers for service %s.\n", len(containers), *serviceName)

	// 2. Pull latest image
	fmt.Printf("Pulling latest image: %s\n", *imageName)
	reader, err := cli.ImagePull(ctx, *imageName, image.PullOptions{})
	if err != nil {
		log.Fatal(err)
	}
	if _, err := io.Copy(os.Stdout, reader); err != nil {
		log.Fatalf("failed to copy to stdout: %v", err)
	}

	// 3. Replace each container one-by-one
	for _, old := range containers {
		fmt.Printf("\n--- Replacing container %s (%s) ---\n", old.ID[:12], strings.Join(old.Names, ", "))

		// Get old container config
		inspect, err := cli.ContainerInspect(ctx, old.ID)
		if err != nil {
			log.Fatal(err)
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
			log.Fatal(err)
		}

		// Start new container
		if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Started new container: %s\n", resp.ID[:12])

		// Wait for healthy
		fmt.Println("Waiting for new container to be healthy...")
		if err := waitForHealthy(cli, resp.ID, time.Duration(*timeoutSeconds)*time.Second, time.Duration(*waitSeconds)*time.Second); err != nil {
			log.Fatalf("New container %s did not become healthy: %v", resp.ID[:12], err)
		}
		fmt.Printf("✅ New container %s is healthy!\n", resp.ID[:12])

		// Stop & remove old container
		fmt.Printf("Stopping old container: %s\n", old.ID[:12])
		cli.ContainerStop(ctx, old.ID, container.StopOptions{})
		cli.ContainerRemove(ctx, old.ID, container.RemoveOptions{})
		fmt.Println("Old container removed.")
	}

	fmt.Println("\nRolling restart completed successfully.")
}

func waitForHealthy(cli *client.Client, containerID string, timeout time.Duration, interval time.Duration) error {
	ctx := context.Background()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		inspect, err := cli.ContainerInspect(ctx, containerID)
		if err != nil {
			return err
		}
		if inspect.State != nil && inspect.State.Health != nil {
			if inspect.State.Health.Status == "healthy" {
				return nil
			}
		}
		time.Sleep(interval)
	}
	return fmt.Errorf("timeout waiting for container %s to be healthy", containerID)
}
