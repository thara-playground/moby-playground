package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

func main() {
	ctx := context.Background()

	cli, err := client.New(client.FromEnv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create docker client: %v\n", err)
		os.Exit(1)
	}
	defer cli.Close()

	if err := run(ctx, cli); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cli *client.Client) error {
	if err := showDaemonInfo(ctx, cli); err != nil {
		return fmt.Errorf("daemon info: %w", err)
	}
	if err := showComposeContainers(ctx, cli); err != nil {
		return fmt.Errorf("compose containers: %w", err)
	}
	if err := showImages(ctx, cli); err != nil {
		return fmt.Errorf("images: %w", err)
	}
	if err := showNetworks(ctx, cli); err != nil {
		return fmt.Errorf("networks: %w", err)
	}
	return nil
}

func showDaemonInfo(ctx context.Context, cli *client.Client) error {
	fmt.Println("=== Docker Daemon Info ===")

	ping, err := cli.Ping(ctx, client.PingOptions{})
	if err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	fmt.Printf("Ping OK — API Version: %s\n", ping.APIVersion)

	info, err := cli.Info(ctx, client.InfoOptions{})
	if err != nil {
		return fmt.Errorf("info: %w", err)
	}

	ver, err := cli.ServerVersion(ctx, client.ServerVersionOptions{})
	if err != nil {
		return fmt.Errorf("server version: %w", err)
	}

	fmt.Printf("Server Version: %s\n", ver.Version)
	fmt.Printf("OS/Arch:        %s/%s\n", info.Info.OSType, info.Info.Architecture)
	fmt.Printf("Containers:     %d (running: %d, paused: %d, stopped: %d)\n",
		info.Info.Containers, info.Info.ContainersRunning, info.Info.ContainersPaused, info.Info.ContainersStopped)
	fmt.Printf("Images:         %d\n", info.Info.Images)
	fmt.Println()

	return nil
}

func showComposeContainers(ctx context.Context, cli *client.Client) error {
	fmt.Println("=== Compose Containers (moby-playground) ===")

	f := client.Filters{}.Add("label", "com.docker.compose.project=moby-playground")

	result, err := cli.ContainerList(ctx, client.ContainerListOptions{
		All:     true,
		Filters: f,
	})
	if err != nil {
		return fmt.Errorf("container list: %w", err)
	}

	if len(result.Items) == 0 {
		fmt.Println("No containers found. Run 'docker compose up -d' first.")
		fmt.Println()
		return nil
	}

	for _, c := range result.Items {
		service := c.Labels["com.docker.compose.service"]
		fmt.Printf("--- %s ---\n", service)
		fmt.Printf("  ID:     %.12s\n", c.ID)
		fmt.Printf("  Image:  %s\n", c.Image)
		fmt.Printf("  State:  %s (%s)\n", c.State, c.Status)

		if len(c.Ports) > 0 {
			var ports []string
			for _, p := range c.Ports {
				if p.PublicPort != 0 {
					ports = append(ports, fmt.Sprintf("%v:%d->%d/%s", p.IP, p.PublicPort, p.PrivatePort, p.Type))
				} else {
					ports = append(ports, fmt.Sprintf("%d/%s", p.PrivatePort, p.Type))
				}
			}
			fmt.Printf("  Ports:  %s\n", strings.Join(ports, ", "))
		}

		inspect, err := cli.ContainerInspect(ctx, c.ID, client.ContainerInspectOptions{})
		if err != nil {
			fmt.Printf("  Health: (inspect failed: %v)\n", err)
		} else if inspect.Container.State != nil && inspect.Container.State.Health != nil {
			fmt.Printf("  Health: %s\n", inspect.Container.State.Health.Status)
		} else {
			fmt.Printf("  Health: (no healthcheck)\n")
		}

		appLabels := filterLabels(c.Labels, "app.")
		if len(appLabels) > 0 {
			fmt.Printf("  Labels:\n")
			for _, kv := range appLabels {
				fmt.Printf("    %s = %s\n", kv[0], kv[1])
			}
		}

		if c.State == "running" {
			statsResult, err := cli.ContainerStats(ctx, c.ID, client.ContainerStatsOptions{
				IncludePreviousSample: true,
			})
			if err != nil {
				fmt.Printf("  CPU:    (stats failed: %v)\n", err)
			} else {
				var stats container.StatsResponse
				if err := json.NewDecoder(statsResult.Body).Decode(&stats); err != nil {
					fmt.Printf("  CPU:    (decode failed: %v)\n", err)
				} else {
					cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
					systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
					cpuPercent := 0.0
					if systemDelta > 0 && cpuDelta > 0 {
						cpuPercent = (cpuDelta / systemDelta) * float64(stats.CPUStats.OnlineCPUs) * 100.0
					}

					memUsage := float64(stats.MemoryStats.Usage)
					memLimit := float64(stats.MemoryStats.Limit)
					memPercent := 0.0
					if memLimit > 0 {
						memPercent = (memUsage / memLimit) * 100.0
					}

					fmt.Printf("  CPU:    %.2f%%\n", cpuPercent)
					fmt.Printf("  Memory: %.1f MiB / %.1f GiB (%.2f%%)\n",
						memUsage/1024/1024, memLimit/1024/1024/1024, memPercent)
				}
				statsResult.Body.Close()
			}
		}
		fmt.Println()
	}

	return nil
}

func filterLabels(labels map[string]string, prefix string) [][2]string {
	var result [][2]string
	for k, v := range labels {
		if strings.HasPrefix(k, prefix) {
			result = append(result, [2]string{k, v})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i][0] < result[j][0]
	})
	return result
}

func showImages(ctx context.Context, cli *client.Client) error {
	fmt.Println("=== Local Images ===")

	result, err := cli.ImageList(ctx, client.ImageListOptions{})
	if err != nil {
		return fmt.Errorf("image list: %w", err)
	}

	for _, img := range result.Items {
		tags := "<none>"
		if len(img.RepoTags) > 0 {
			tags = strings.Join(img.RepoTags, ", ")
		}
		sizeMB := float64(img.Size) / 1024 / 1024
		fmt.Printf("  %-40s  %.1f MB\n", tags, sizeMB)
	}
	fmt.Println()

	return nil
}

func showNetworks(ctx context.Context, cli *client.Client) error {
	fmt.Println("=== Networks ===")

	result, err := cli.NetworkList(ctx, client.NetworkListOptions{})
	if err != nil {
		return fmt.Errorf("network list: %w", err)
	}

	for _, n := range result.Items {
		fmt.Printf("  %-30s  driver=%-10s  scope=%s\n", n.Name, n.Driver, n.Scope)
	}
	fmt.Println()

	return nil
}
