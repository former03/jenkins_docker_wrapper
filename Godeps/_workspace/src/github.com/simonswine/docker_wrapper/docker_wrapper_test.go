package docker_wrapper

import (
	"github.com/fsouza/go-dockerclient"
	"github.com/simonswine/docker_wrapper/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"testing"
)

type DockerWrapperTestSuite struct {
	suite.Suite
	dw     DockerWrapper
	client *mocks.DockerClientInterface
}

// before each test
func (suite *DockerWrapperTestSuite) SetupTest() {
	// create mock client
	suite.client = new(mocks.DockerClientInterface)
	suite.dw.client = suite.client

	// create mock container
	var container docker.Container
	container.ID = "c1"
	suite.dw.container = &container
}

func (suite *DockerWrapperTestSuite) TestRunCommandSuccessful() {

	// handle create exec
	var c_retval docker.Exec
	c_retval.ID = "e1"
	suite.client.On(
		"CreateExec",
		mock.AnythingOfType("docker.CreateExecOptions"),
	).Return(
		&c_retval,
		nil,
	).Once()

	// handle start exec
	suite.client.On(
		"StartExec",
		c_retval.ID,
		mock.AnythingOfType("docker.StartExecOptions"),
	).Return(nil).Once()

	// handle inspect exec
	var i_retval docker.ExecInspect
	i_retval.ExitCode = 1328
	suite.client.On(
		"InspectExec",
		c_retval.ID,
	).Return(&i_retval, nil).Once()

	stdout, stderr, retval, _ := suite.dw.RunCommand([]string{"test-command", "arg1"})

	suite.client.AssertExpectations(suite.T())

	// test args of create exec
	c_args := suite.client.Calls[0].Arguments.Get(0).(docker.CreateExecOptions)
	suite.Equal(c_args.Cmd, []string{"test-command", "arg1"})

	suite.Equal("stderr", stderr)
	suite.Equal("output", stdout)
	suite.Equal(1328, retval)
}

// map suite
func TestDockerWrapperTestSuite(t *testing.T) {
	suite.Run(t, new(DockerWrapperTestSuite))
}
