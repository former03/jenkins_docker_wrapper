package main

import (
	"github.com/docker/docker/api/client"
	"github.com/fsouza/go-dockerclient"
	"gopkg.in/alecthomas/kingpin.v1"
	"log"
	"os"
)

type Arguments struct {
	debug        *bool   // Debug mode flag
	projekt_conf *bool   // Detect image_name from projekt_conf
	image_name   *string // Image name of docker image
	no_rm        *bool   // Don't remove container after execution
}

type Config struct {
	secure_path        []string // Secure location to mount
	default_shell      string
	cleanup_containers []string // Containers to remove at the end
}

var version = "0.0.1"

var config Config

var args Arguments

var docker_client *docker.Client

// ensure cleanup of all ressources
func cleanup() {
	cleanup_containers()
}

// parse and validate command line arguments
func parse_arguments() {
	args.debug = kingpin.Flag("debug", "Enable debug mode.").Short('d').Bool()
	args.projekt_conf = kingpin.Flag("projekt_conf", "Parse projekt.conf for image name.").Short('p').Bool()
	args.image_name = kingpin.Flag("image_name", "Image name of docker image.").Short('i').String()
	args.no_rm = kingpin.Flag("no_rm", "Don't remove container after execution.").Short('n').Bool()

	kingpin.Version(version)
	kingpin.Parse()
	if *args.debug {
		log.Printf("debug mode enabled")
	}
}

// parse and validate local config
func parse_config() {

}

// connect to docker
func connect_docker() *docker.Client {
	endpoint := "unix:///var/run/docker.sock"
	client, _ := docker.NewClient(endpoint)
	version, err := client.Version()
	if err != nil {
		log.Panicf("Docker connection not successful: %s", err)
	}
	if *args.debug {
		log.Printf("Docker connection successful. server version: %s\n", version.Get("Version"))
	}
	return client
}

// container error message
func handle_error_container(msg string, id string, err error) {
	if err != nil {
		log.Panicf("Docker %s container (id=%s) not successful: %s", msg, id, err)
	}
	if *args.debug {
		log.Printf("Docker %s container (id=%s) successful", msg, id)
	}
}

// cleanup containers
func cleanup_containers() {
	for _, id := range config.cleanup_containers {
		err := remove_container(id)
		handle_error_container("removal of", id, err)
	}
}

// run a container
func run_container(command []string) (returncode int) {

	// create
	container, err := create_container(command)
	if err != nil {
		log.Panicf("Docker creation of container not successful: %s", err)
	}
	if *args.debug {
		handle_error_container("creation of", container.ID, err)
	}

	// add container for removal
	config.cleanup_containers = append(config.cleanup_containers, container.ID)

	// start container
	err = docker_client.StartContainer(container.ID, get_host_config())
	handle_error_container("start of", container.ID, err)

	// attach to container
	err = attach_to_container(container.ID)
	handle_error_container("attachment of", container.ID, err)

	// stop container
	docker_client.StopContainer(container.ID, 2)

	// wait for container
	returncode, _ = docker_client.WaitContainer(container.ID)
	return returncode

}

// generate host config
func get_host_config() *docker.HostConfig {
	var config docker.HostConfig
	config.RestartPolicy = docker.NeverRestart()
	return &config
}

// create the container
func create_container(cmd []string) (container *docker.Container, err error) {
	var c_config docker.Config
	c_config.Image = *args.image_name
	c_config.Cmd = cmd
	c_config.Tty = true
	c_config.OpenStdin = true

	host_config := get_host_config()

	var copts docker.CreateContainerOptions
	copts.Name = "testcontainer1"
	copts.Config = &c_config
	copts.HostConfig = host_config

	return docker_client.CreateContainer(copts)
}

// attach to a container
func attach_to_container(id string) (err error) {
	var opts docker.AttachToContainerOptions
	opts.Container = id
	opts.Stdin = true
	opts.Stdout = true
	opts.Stderr = true
	opts.InputStream = os.Stdin
	opts.OutputStream = os.Stdout
	opts.ErrorStream = os.Stderr
	opts.Stream = true
	opts.RawTerminal = true
	return docker_client.AttachToContainer(opts)
}

// remove a container
func remove_container(id string) (err error) {
	var opts docker.RemoveContainerOptions
	opts.ID = id
	return docker_client.RemoveContainer(opts)
}

// set default config
func set_default_config() {
	config.default_shell = "/bin/bash"
	config.cleanup_containers = []string{}
}

// main function
func main() {
	defer cleanup()

	cli := client.NewDockerCli(os.Stdin, os.Stdout, os.Stderr, "no", "unix", "/var/run/docker.sock", nil)
	err := cli.CmdRun("-t", "-i", "--rm", "ubuntu", "/bin/bash")
	log.Printf("%s", err)
	/*
		set_default_config()

		parse_arguments()

		docker_client = connect_docker()

		run_container([]string{config.default_shell})
	*/
}
