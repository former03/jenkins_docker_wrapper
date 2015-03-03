package main

import (
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
	"gopkg.in/alecthomas/kingpin.v1"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Arguments struct {
	debug        *bool   // Debug mode flag
	projekt_conf *bool   // Detect image_name from projekt_conf
	image_name   *string // Image name of docker image
	no_rm        *bool   // Don't remove container after execution
}

type Config struct {
	my_args            []string // Arguments for me
	container_args     []string // Arguments for the container shell
	files_to_copy      []string // Files to copy to the container
	basename           string   // Base name of executable
	secure_path        []string // Secure location to mount
	default_shell      string
	job_name           string
	build_id           int
	jenkins_user       string
	jenkins_workspace  string
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

// parse cli arguments
func parse_arguments(in_args []string) {
	// get basename of me
	config.basename = filepath.Base(in_args[0])

	// split arguments
	config.my_args, config.container_args = split_arguments(config.basename, in_args[1:])

	// parse my arguments
	args = parse_my_arguments(config.basename, config.my_args)
}

// test if legacy parser is needed
func parse_arguments_legacy(basename string) bool {
	if basename == "jenkins_docker_run" {
		return true
	}
	return false
}

// split up command line arguments
func split_arguments(basename string, cli_args []string) (my []string, container []string) {
	seperator := "--"
	seperator_pos := -1
	legacy_my_args := []string{}
	legacy := parse_arguments_legacy(basename)

	// handle legacy arguments
	if legacy {
		legacy_my_args = append(legacy_my_args, cli_args[0])
		seperator_pos = 0
	} else {
		for index, elem := range cli_args {
			if elem == seperator {
				seperator_pos = index
				break
			}
		}
	}

	if seperator_pos >= 0 {
		log.Debugf("Found seperator '%s' on position=%d", seperator, seperator_pos)
		my = cli_args[0:seperator_pos]
		container = cli_args[seperator_pos+1:]
	} else {
		my = []string{}
		container = cli_args
	}

	if legacy {
		my = legacy_my_args
	}

	log.Debugf("My arguments       : %s", my)
	log.Debugf("Container arguments: %s", container)
	return my, container
}

// parse and validate my command line arguments
func parse_my_arguments(basename string, cli_args []string) Arguments {

	var args Arguments

	parser := kingpin.New(basename, "")

	args.debug = parser.Flag("debug", "Enable debug mode.").Short('d').Bool()
	args.projekt_conf = parser.Flag("projekt_conf", "Parse projekt.conf for image name.").Short('p').Bool()
	args.image_name = parser.Flag("image_name", "Image name of docker image.").Short('i').String()
	args.no_rm = parser.Flag("no_rm", "Don't remove container after execution.").Short('n').Bool()

	if parse_arguments_legacy(basename) {
		args.image_name = &cli_args[0]
		cli_args = []string{}
	}

	parser.Version(version)
	parser.Parse(cli_args)

	if *args.debug {
		log.SetLevel(log.DebugLevel)
	}

	return args
}

func build_environment_blacklist(key string, value string) (additional []string, err error) {
	return []string{}, nil
}

func build_environment_validate_user(key string, value string) (additional []string, err error) {
	if value == config.jenkins_user {
		return []string{fmt.Sprintf("%s=%s", key, value)}, err
	}
	err = errors.New(fmt.Sprintf("Invalid user environment '%s', expect to be '%s'", value, config.jenkins_user))
	return []string{}, err
}

func build_environment_validate_workspace(key string, value string) (additional []string, err error) {

	if !filepath.IsAbs(value) {
		err := errors.New(fmt.Sprintf("Invalid path in %s '%s', expected to be absolute path", key, value))
		return []string{}, err
	}

	path, err := filepath.Abs(value)
	if err != nil {
		return []string{}, err
	}

	if !strings.HasPrefix(path, config.jenkins_workspace) {
		err := errors.New(fmt.Sprintf("Invalid path in %s '%s', expected to be within %s", key, path, config.jenkins_workspace))
		return []string{}, err
	}

	return []string{fmt.Sprintf("%s=%s", key, path)}, err
}

func build_environment_validate_ssh_auth_sock(key string, value string) (additional []string, err error) {

	if !filepath.IsAbs(value) {
		err := errors.New(fmt.Sprintf("Invalid path in %s '%s', expected to be absolute path", key, value))
		return []string{}, err
	}

	// append ssh socket to copy slice
	config.files_to_copy = append(config.files_to_copy, value)

	return []string{fmt.Sprintf("%s=%s", key, value)}, err
}

func build_environment_store_build_id(key string, value string) (additional []string, err error) {
	i, err := strconv.Atoi(value)
	if err != nil {
		return []string{}, nil
	}
	config.build_id = i
	return []string{fmt.Sprintf("%s=%s", key, value)}, err
}

func build_environment_store_job_name(key string, value string) (additional []string, err error) {
	config.job_name = value
	return []string{fmt.Sprintf("%s=%s", key, value)}, err
}

// filter and check environment
func build_environment(env []string) (output []string, err error) {

	output = []string{}

	// create handler map
	var m map[string]func(string, string) ([]string, error)
	m = make(map[string]func(string, string) ([]string, error))

	// blacklist following keys
	m["SSH_CLIENT"] = build_environment_blacklist
	m["SSH_CONNECTION"] = build_environment_blacklist
	m["LD_LIBRARY_PATH"] = build_environment_blacklist

	// validations
	m["USER"] = build_environment_validate_user
	m["PWD"] = build_environment_validate_workspace
	m["WORKSPACE"] = build_environment_validate_workspace

	// validate and move into container
	m["SSH_AUTH_SOCK"] = build_environment_validate_ssh_auth_sock

	// store build id
	m["BUILD_ID"] = build_environment_store_build_id

	// store project name
	m["JOB_NAME"] = build_environment_store_job_name

	for _, env_elem := range env {

		// split environment
		env_split := strings.SplitN(env_elem, "=", 2)
		if len(env_split) != 2 {
			log.Warnf("Can't parse env: %s", env_elem)
			continue
		}
		key, value := env_split[0], env_split[1]

		//.check if key has handler
		if val, ok := m[key]; ok {
			additional, err := val(key, value)
			if err != nil {
				return []string{}, err
			}
			// if addtional env variable exists add it
			if len(additional) > 0 {
				output = append(output, additional...)
			}
			continue
		}

		// add to output
		output = append(output, fmt.Sprintf("%s=%s", key, value))
	}
	return output, err
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
	log.Debugf("Docker connection successful. server version: %s\n", version.Get("Version"))
	return client
}

// container error message
func handle_error_container(msg string, id string, err error) {
	if err != nil {
		log.Panicf("Docker %s container (id=%s) not successful: %s", msg, id, err)
	}
	log.Debugf("Docker %s container (id=%s) successful", msg, id)
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
	handle_error_container("creation of", container.ID, err)

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

	log.Debugf("Create options image=%s command=%s", c_config.Image, c_config.Cmd)
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
func initialize() {

	// set log level
	log.SetLevel(log.DebugLevel)

	// set default shell to bash
	config.default_shell = "/bin/bash"
	config.jenkins_user = "jenkins"
	config.jenkins_workspace = "/jenkins/workspace"

	config.cleanup_containers = []string{}

	parse_arguments(os.Args)

	build_environment(os.Environ())

}

// main function
func main() {
	defer cleanup()

	initialize()

	docker_client = connect_docker()

	run_container([]string{config.default_shell})
}
