package services

// For orchestrating the local Docker node API service

import (
	"YourPlace/src/core"
	"context"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

var dockerClient *client.Client

func DockerInit() {
	_dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		core.LogError("Could not create Docker client while trying to list containers: " + err.Error())
	}
	defer _dockerClient.Close()
	dockerClient = _dockerClient
}
func DockerPing() bool {
	if dockerClient == nil {
		DockerInit()
	}
	_, err := dockerClient.Ping(context.Background())
	if err != nil {
		return false
	}
	return true
}
func DockerListContainers() ([]string, error) {
	if dockerClient == nil {
		DockerInit()
	}
	containers, err := dockerClient.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		return nil, core.LogDebugReturn("Could not list Docker containers: " + err.Error())
	}
	containerNames := make([]string, len(containers))
	for i, container := range containers {
		containerNames[i] = container.Names[0][1:]
	}
	return containerNames, nil
}
