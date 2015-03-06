package main

import (
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/simonswine/docker_wrapper"
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
	cleanup_containers []string                        // Containers to remove at the end
	wrappers           *[]docker_wrapper.DockerWrapper // Docker wrappers
}

var version = "0.0.1"

var config Config

var args Arguments

// ensure cleanup of all ressources
func cleanup() {
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

	dw, err := docker_wrapper.New()
	if err != nil {
		log.Panic(err)
	}

	dw.ImageName = *args.image_name
	// Starting the docker container
	err = dw.Run()
	if err != nil {
		log.Fatalf("Docker error: %s", err)
	}

	stdout, stderr, ret_val, _ := dw.RunCommand([]string{"groupadd", "-g", "1000", "jenkins"})
	stdout, stderr, ret_val, _ = dw.RunCommand([]string{"useradd", "-d", "/jenkins", "-g", "1000", "-u", "1000", "jenkins"})
	fmt.Printf("stdout=%s stderr=%s ret_val=%d", stdout, stderr, ret_val)
}
