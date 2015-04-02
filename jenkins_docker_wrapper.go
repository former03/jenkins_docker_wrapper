package main

import (
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/danryan/go-group/os/group"
	"github.com/former03/docker_wrapper"
	"gopkg.in/alecthomas/kingpin.v1"
	"io"
	"io/ioutil"
	"os"
	"os/user"
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
	tmp_files_to_move  []string // Files to copy to the container
	basename           string   // Base name of executable
	environment        []string // Environment variables for the container
	volumes            []string // Secure location to mount
	default_shell      string
	job_name           string
	build_id           int
	jenkins_user       string
	jenkins_home       string
	workspace_path     string
	tmp_dir            string                          // Container tmp dir
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

	workspace_path := filepath.Join(config.jenkins_home, "workspace")
	if !strings.HasPrefix(path, workspace_path) {
		err := errors.New(fmt.Sprintf("Invalid path in %s '%s', expected to be within %s", key, path, workspace_path))
		return []string{}, err
	}

	config.workspace_path = path

	return []string{fmt.Sprintf("%s=%s", key, path)}, err
}

func build_environment_validate_ssh_auth_sock(key string, value string) (additional []string, err error) {

	if !filepath.IsAbs(value) {
		err := errors.New(fmt.Sprintf("Invalid path in %s '%s', expected to be absolute path", key, value))
		return []string{}, err
	}

	// append ssh socket to copy slice
	config.tmp_files_to_move = append(config.tmp_files_to_move, value)

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
	m["PATH"] = build_environment_blacklist
	m["NVM_DIR"] = build_environment_blacklist
	m["NVM_NODEJS_ORG_MIRROR"] = build_environment_blacklist
	m["LANG"] = build_environment_blacklist

	// validations
	m["USER"] = build_environment_validate_user
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

func create_tmp_dir() error {
	// TODO: Cleanup container tempdir
	temp_dir, err := ioutil.TempDir("", "jenkins_docker_wrapper")
	if err != nil {
		return err
	}
	config.tmp_dir = temp_dir
	log.Debugf("Created container temp dir in %s", temp_dir)

	err = os.Chmod(temp_dir, 0777)
	if err != nil {
		return err
	}

	for i := range config.tmp_files_to_move {
		src := config.tmp_files_to_move[i]
		dest := filepath.Join(temp_dir, filepath.Base(src))
		log.Debugf("Moving %s to %s", src, dest)
		err := os.Rename(src, dest)
		if err != nil {
			return err
		}
	}

	return nil
}

func copy_file(src, dst string) (int64, error) {
	src_file, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer src_file.Close()

	src_file_stat, err := src_file.Stat()
	if err != nil {
		return 0, err
	}

	if !src_file_stat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	dst_file, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer dst_file.Close()
	return io.Copy(dst_file, src_file)
}

func copy_ssh_known_hosts() error {
	source := filepath.Join(os.Getenv("HOME"), ".ssh/known_hosts")
	dest := filepath.Join(config.tmp_dir, "known_hosts")
	log.Debugf("Copy ssh known hosts from '%s' to '%s'", source, dest)
	_, err := copy_file(source, dest)
	return err
}

// set default config
func initialize() error {

	// set log level
	log.SetLevel(log.DebugLevel)

	// set default shell to bash
	config.default_shell = "/bin/bash"
	config.jenkins_user = "jenkins"
	config.jenkins_home = "/jenkins"

	config.cleanup_containers = []string{}

	parse_arguments(os.Args)

	// evaluate environment
	env, err := build_environment(os.Environ())
	if err != nil {
		return err
	}

	// add utf8 language env
	env = append(env, "LANG=C.UTF-8")

	config.environment = env
	for i := range env {
		log.Debugf("container env var: %s", env[i])
	}

	// add workspace to volumes
	config.volumes = append(
		config.volumes,
		fmt.Sprintf(
			"%s:%s",
			config.workspace_path,
			config.workspace_path,
		),
	)

	// append script to the files to copy
	if n := len(config.container_args); n > 0 {
		config.tmp_files_to_move = append(config.tmp_files_to_move, config.container_args[n-1])
	}

	// create containers temp dir
	err = create_tmp_dir()
	if err != nil {
		return err
	}

	// add tmp dir to volumes
	config.volumes = append(
		config.volumes,
		fmt.Sprintf(
			"%s:/tmp",
			config.tmp_dir,
		),
	)

	// move ssh known hosts into container
	err = copy_ssh_known_hosts()
	if err != nil {
		return err
	}

	return nil

}

// run command
func run_command(dw *docker_wrapper.DockerWrapper, command []string) (string, string, int, error) {
	stdout, stderr, ret_val, err := dw.RunCommand(command)
	log.Debugf("running command=%v ret_val=%d stdout=%s stderr=%s", command, ret_val, stdout, stderr)
	return stdout, stderr, ret_val, err
}

func run_command_expect(dw *docker_wrapper.DockerWrapper, command []string, expect int) (string, string, int, error) {
	stdout, stderr, ret_val, err := run_command(dw, command)
	if ret_val != expect {
		msg := fmt.Sprintf("expected ret_val is %d but received %d command %v ", expect, ret_val, command)
		log.Warn(msg)
		return stdout, stderr, ret_val, errors.New(msg)
	}
	return stdout, stderr, ret_val, err
}

func init_container(dw *docker_wrapper.DockerWrapper) error {
	jenkins_home_path := "/jenkins"
	ssh_dir_path := filepath.Join(jenkins_home_path, ".ssh")
	ssh_known_hosts_path := filepath.Join(ssh_dir_path, "known_hosts")

	username := config.jenkins_user
	user_struct, err := user.Lookup(username)
	if err != nil {
		return err
	}
	uid_str := user_struct.Uid
	gid_str := user_struct.Gid
	group_struct, err := group.LookupGroupId(gid_str)
	if err != nil {
		return err
	}
	groupname := group_struct.Name

	log.Debugf("Detected user=%s (%s) group=%s (%s)", username, uid_str, groupname, gid_str)

	user_group := fmt.Sprintf("%s:%s", username, groupname)

	// remove existing uid
	stdout, _, ret_val, _ := run_command(dw, []string{"getent", "passwd", uid_str})
	if ret_val == 0 {
		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		for i := range lines {
			user := strings.Split(lines[i], ":")
			log.Infof("Remove user '%s'", user[0])
			_, _, _, err := run_command_expect(dw, []string{"userdel", user[0]}, 0)
			if err != nil {
				return err
			}
		}
	}
	// remove existing user
	_, _, ret_val, _ = run_command(dw, []string{"getent", "passwd", username})
	if ret_val == 0 {
		log.Infof("Remove user '%s'", username)
		_, _, _, err := run_command_expect(dw, []string{"userdel", username}, 0)
		if err != nil {
			return err
		}
	}

	// remove existing gid
	stdout, _, ret_val, _ = run_command(dw, []string{"getent", "group", gid_str})
	if ret_val == 0 {
		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		for i := range lines {
			group := strings.Split(lines[i], ":")
			log.Infof("Remove group '%s'", group[0])
			_, _, _, err := run_command_expect(dw, []string{"groupdel", group[0]}, 0)
			if err != nil {
				return err
			}
		}
	}

	// remove existing group
	_, _, ret_val, _ = run_command(dw, []string{"getent", "group", groupname})
	if ret_val == 0 {
		log.Infof("Remove group '%s'", groupname)
		_, _, _, err := run_command_expect(dw, []string{"groupdel", groupname}, 0)
		if err != nil {
			return err
		}
	}

	// add group
	_, _, _, err = run_command_expect(dw, []string{"groupadd", "-g", gid_str, groupname}, 0)
	if err != nil {
		return err
	}

	// add user
	_, _, _, err = run_command_expect(dw, []string{"useradd", "-d", jenkins_home_path, "-g", gid_str, "-u", uid_str, username}, 0)
	if err != nil {
		return err
	}

	// resets sudoers file
	_, _, _, err = run_command_expect(dw, []string{"sh", "-c", "echo \"root ALL=(ALL:ALL) ALL\" > /etc/sudoers"}, 0)
	if err != nil {
		return err
	}

	// move ssh known hosts file
	_, _, _, err = run_command_expect(dw, []string{"mkdir", "-p", ssh_dir_path}, 0)
	if err != nil {
		return err
	}
	_, _, _, err = run_command_expect(dw, []string{"mv", "/tmp/known_hosts", ssh_known_hosts_path}, 0)
	if err != nil {
		return err
	}

	// reset rights in jenkins home
	_, _, _, err = run_command_expect(dw, []string{"chown", "-R", user_group, jenkins_home_path}, 0)
	if err != nil {
		return err
	}

	return nil
}

// main function
func main() {
	defer cleanup()

	err := initialize()
	if err != nil {
		log.Panic(err)
	}

	dw, err := docker_wrapper.New()
	if err != nil {
		log.Panic(err)
	}

	dw.ImageName = *args.image_name
	dw.Volumes = config.volumes
	dw.Environment = config.environment
	dw.WorkingDir = config.workspace_path
	// Starting the docker container
	err = dw.Run()
	if err != nil {
		log.Fatalf("Docker error: %s", err)
	}

	err = init_container(dw)
	if err != nil {
		log.Fatal(err)
	}

	// call jenkins script
	command := []string{"sudo", "-E", "-u", "jenkins", "bash"}
	command = append(command, config.container_args...)
	ret_val, err := dw.RunCommandAttach(command, false)
	if err != nil {
		log.Fatal(err)
	}

	// clean up container
	err = dw.Stop()
	if err != nil {
		log.Warn(err)
	}

	// remove container
	err = dw.Remove()
	if err != nil {
		log.Warn(err)
	}

	os.Exit(ret_val)

}
