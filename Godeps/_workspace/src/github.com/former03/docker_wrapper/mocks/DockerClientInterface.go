package mocks

import "github.com/stretchr/testify/mock"
import "github.com/fsouza/go-dockerclient"

type DockerClientInterface struct {
	mock.Mock
}

func (m *DockerClientInterface) CreateExec(opts docker.CreateExecOptions) (*docker.Exec, error) {
	ret := m.Called(opts)

	r0 := ret.Get(0).(*docker.Exec)
	r1 := ret.Error(1)

	return r0, r1
}
func (m *DockerClientInterface) StartExec(id string, opts docker.StartExecOptions) error {
	ret := m.Called(id, opts)

	if opts.ErrorStream != nil {
		opts.ErrorStream.Write([]byte("stderr"))
	}
	if opts.OutputStream != nil {
		opts.OutputStream.Write([]byte("output"))
	}

	r0 := ret.Error(0)

	return r0
}
func (m *DockerClientInterface) InspectExec(id string) (*docker.ExecInspect, error) {
	ret := m.Called(id)

	r0 := ret.Get(0).(*docker.ExecInspect)
	r1 := ret.Error(1)

	return r0, r1
}
func (m *DockerClientInterface) StartContainer(id string, hostConfig *docker.HostConfig) error {
	ret := m.Called(id, hostConfig)

	r0 := ret.Error(0)

	return r0
}
func (m *DockerClientInterface) StopContainer(id string, timeout uint) error {
	ret := m.Called(id, timeout)

	r0 := ret.Error(0)

	return r0
}
func (m *DockerClientInterface) WaitContainer(id string) (int, error) {
	ret := m.Called(id)

	r0 := ret.Get(0).(int)
	r1 := ret.Error(1)

	return r0, r1
}
func (m *DockerClientInterface) CreateContainer(opts docker.CreateContainerOptions) (*docker.Container, error) {
	ret := m.Called(opts)

	r0 := ret.Get(0).(*docker.Container)
	r1 := ret.Error(1)

	return r0, r1
}
func (m *DockerClientInterface) RemoveContainer(opts docker.RemoveContainerOptions) error {
	ret := m.Called(opts)

	r0 := ret.Error(0)

	return r0
}
func (m *DockerClientInterface) AttachToContainer(opts docker.AttachToContainerOptions) error {
	ret := m.Called(opts)

	r0 := ret.Error(0)

	return r0
}
