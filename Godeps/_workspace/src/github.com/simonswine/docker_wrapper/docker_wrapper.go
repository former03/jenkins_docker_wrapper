package docker_wrapper

import (
	"bytes"
	log "github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
	"os"
)

type DockerWrapper struct {
	client        DockerClientInterface
	DefaultRunCmd []string
	Volumes       []string
	ImageName     string
	ContainerName string
	WorkingDir    string
	Environment   []string
	container     *docker.Container
}

type DockerClientInterface interface {
	CreateExec(opts docker.CreateExecOptions) (*docker.Exec, error)
	StartExec(id string, opts docker.StartExecOptions) error
	InspectExec(id string) (*docker.ExecInspect, error)
	StartContainer(id string, hostConfig *docker.HostConfig) error
	StopContainer(id string, timeout uint) error
	WaitContainer(id string) (int, error)
	CreateContainer(opts docker.CreateContainerOptions) (*docker.Container, error)
	RemoveContainer(opts docker.RemoveContainerOptions) error
	AttachToContainer(opts docker.AttachToContainerOptions) error
}

func New() (*DockerWrapper, error) {
	endpoint := "unix:///var/run/docker.sock"
	client, _ := docker.NewClient(endpoint)
	version, err := client.Version()
	if err != nil {
		log.Panicf("Docker connection not successful: %s", err)
	}
	log.Debugf("Docker connection successful. server version: %s\n", version.Get("Version"))
	return &DockerWrapper{
		client:        client,
		DefaultRunCmd: []string{"cat"},
		ContainerName: "",
	}, nil
}

func (dw *DockerWrapper) RunCommandRetval(e_id string) int {
	// get return code
	execInspect, err := dw.client.InspectExec(e_id)
	if err != nil {
		return -1
	} else {
		return execInspect.ExitCode
	}
}

func (dw *DockerWrapper) RunCommandAttach(command []string, tty bool) (ret_val int, err error) {
	create_config := docker.CreateExecOptions{
		Container:    dw.container.ID,
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          tty,
		Cmd:          command,
	}

	execObj, err := dw.client.CreateExec(create_config)
	if err != nil {
		return -1, err
	}

	start_config := docker.StartExecOptions{
		InputStream:  os.Stdin,
		OutputStream: os.Stdout,
		ErrorStream:  os.Stderr,
		Detach:       false,
		RawTerminal:  false,
		Tty:          tty,
	}
	err = dw.client.StartExec(execObj.ID, start_config)
	if err != nil {
		return -1, err
	}

	return dw.RunCommandRetval(execObj.ID), nil

}

func (dw *DockerWrapper) RunCommand(command []string) (stdout string, stderr string, ret_val int, err error) {
	// prepare container
	create_config := docker.CreateExecOptions{
		Container:    dw.container.ID,
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
		Cmd:          command,
	}

	execObj, err := dw.client.CreateExec(create_config)
	if err != nil {
		log.Fatal(err)
	}

	buf_stdout := new(bytes.Buffer)
	buf_stderr := new(bytes.Buffer)

	start_config := docker.StartExecOptions{
		OutputStream: buf_stdout,
		ErrorStream:  buf_stderr,
		RawTerminal:  false,
		Tty:          false,
	}

	err = dw.client.StartExec(execObj.ID, start_config)
	if err != nil {
		return "", "", -1, err
	}

	ret_val = dw.RunCommandRetval(execObj.ID)

	return buf_stdout.String(), buf_stderr.String(), ret_val, err
}

// run a always running container
func (dw *DockerWrapper) Run() (err error) {

	// create
	dw.container, err = dw.create()
	if err != nil {
		return err
	}

	// start container
	err = dw.client.StartContainer(dw.container.ID, dw.get_host_config())
	if err != nil {
		return err
	}

	return nil
}

func (dw *DockerWrapper) Stop() error {

	// stop container
	dw.client.StopContainer(dw.container.ID, 2)

	// wait for container
	dw.client.WaitContainer(dw.container.ID)
	return nil

}

// generate host config
func (dw *DockerWrapper) get_host_config() *docker.HostConfig {
	var config docker.HostConfig
	config.Binds = dw.Volumes
	config.RestartPolicy = docker.NeverRestart()
	return &config
}

// create the container
func (dw *DockerWrapper) create() (container *docker.Container, err error) {
	var c_config docker.Config
	c_config.Image = dw.ImageName
	c_config.Cmd = dw.DefaultRunCmd
	c_config.WorkingDir = dw.WorkingDir
	c_config.Tty = true
	c_config.OpenStdin = true
	c_config.Env = dw.Environment
	host_config := dw.get_host_config()

	var copts docker.CreateContainerOptions
	if dw.ContainerName != "" {
		copts.Name = "testcontainer1"
	}

	copts.Config = &c_config
	copts.HostConfig = host_config

	log.Debugf("Create options image=%s command=%s", c_config.Image, c_config.Cmd)
	return dw.client.CreateContainer(copts)
}

// remove a container
func (dw *DockerWrapper) Remove() (err error) {
	var opts docker.RemoveContainerOptions
	opts.ID = dw.container.ID
	return dw.client.RemoveContainer(opts)
}
