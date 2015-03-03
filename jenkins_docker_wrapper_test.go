package main

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

// helper to find string in slice
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// Tests new arguments with -- as seperator
func TestArgumentsNew(t *testing.T) {

	// check if legacy parsing is enabled
	assert.Equal(t, false, parse_arguments_legacy("nolegacy"), "Parse legagcy is activated")

	// empty arguments
	parse_arguments([]string{"/tmp/myname"})
	assert.Equal(t, config.basename, "myname", "Basename not correct")

	// test splitting with both before and after
	m, c := split_arguments("myname", []string{"before", "--", "after"})
	assert.Equal(t, m, []string{"before"}, "My arguments not correct")
	assert.Equal(t, c, []string{"after"}, "Container arguments not correct")

	// test splitting with only before and after
	m, c = split_arguments("myname", []string{"--", "after"})
	assert.Equal(t, m, []string{}, "My arguments not correct")
	assert.Equal(t, c, []string{"after"}, "Container arguments not correct")

	m, c = split_arguments("myname", []string{"only"})
	assert.Equal(t, m, []string{}, "My arguments not correct")
	assert.Equal(t, c, []string{"only"}, "Container arguments not correct")

	m, c = split_arguments("myname", []string{"before", "--"})
	assert.Equal(t, m, []string{"before"}, "My arguments not correct")
	assert.Equal(t, c, []string{}, "Container arguments not correct")

	// check if image_name is parsed
	parse_arguments([]string{"myname", "-i", "myimage/test1", "--", "-s", "myscript1"})
	assert.Equal(t, "myimage/test1", *args.image_name, "Image name is not correct")
	assert.Equal(t, config.container_args, []string{"-s", "myscript1"}, "Container arguments not correct")

	// check if image_name is parsed
	parse_arguments([]string{"myname", "--image_name", "myimage/test2", "--", "myscript2"})
	assert.Equal(t, "myimage/test2", *args.image_name, "Image name is not correct")
	assert.Equal(t, config.container_args, []string{"myscript2"}, "Container arguments not correct")

}

// Tests old style arguments
func TestArgumentsLegacy(t *testing.T) {

	my_basename := "jenkins_docker_run"

	// check if legagcy parsing is enabled
	assert.Equal(t, true, parse_arguments_legacy(my_basename), "Parse legagcy is not activated")

	// test parsing of image name
	parse_arguments([]string{my_basename, "myimage/test1", "-x", "after"})
	assert.Equal(t, "myimage/test1", *args.image_name, "Image name is not correct")
	assert.Equal(t, config.container_args, []string{"-x", "after"}, "Container arguments not correct")

}

// Tests old style arguments
func TestEnvironmentBuildBlacklist(t *testing.T) {

	env, err := build_environment([]string{
		"NORMAL=vaLUE",
		"SSH_CONNECTION=FILTER ME, PLEASE!",
	})
	assert.Equal(t, nil, err, "Doesn't return a error")
	assert.Equal(
		t,
		[]string{
			"NORMAL=vaLUE",
		},
		env,
		"Blacklisted env vars are not filtered correctly",
	)
}

func TestEnvironmentBuildUser(t *testing.T) {
	valid_user := "jenkins"
	config.jenkins_user = valid_user

	env, err := build_environment([]string{
		fmt.Sprintf("USER=%s", valid_user),
	})
	assert.Equal(t, nil, err, "Doesn't return a error")
	assert.Equal(
		t,
		[]string{
			fmt.Sprintf("USER=%s", valid_user),
		},
		env,
		"Valid user doesn't get filtered",
	)

	env, err = build_environment([]string{
		"USER=invalid",
	})
	assert.NotEqual(t, nil, err, "Do return a error")
	assert.Equal(
		t,
		[]string{},
		env,
		"Error build has to return empty array",
	)
}

func TestEnvironmentBuildWorkspace(t *testing.T) {
	valid_path := "/jenkins/workspace"
	config.jenkins_workspace = valid_path

	// Validate workspace pwd directory
	for _, key := range []string{"PWD", "WORKSPACE"} {

		env, err := build_environment([]string{
			fmt.Sprintf("%s=%s/kunde1", key, valid_path),
		})
		assert.Equal(t, nil, err, "Do not return a error")
		assert.Equal(
			t,
			[]string{
				fmt.Sprintf("%s=%s/kunde1", key, valid_path),
			},
			env,
			"Has to accept correct path",
		)

		env, err = build_environment([]string{
			fmt.Sprintf("%s=%s/kunde1/../kunde2", key, valid_path),
		})
		assert.Equal(t, nil, err, "Do not return a error")
		assert.Equal(
			t,
			[]string{
				fmt.Sprintf("%s=%s/kunde2", key, valid_path),
			},
			env,
			"Has to accept hacks with correct path",
		)

		env, err = build_environment([]string{
			fmt.Sprintf("%s=%s/kunde2/down", key, valid_path),
		})
		assert.Equal(t, nil, err, "Do not return a error")
		assert.Equal(
			t,
			[]string{
				fmt.Sprintf("%s=%s/kunde2/down", key, valid_path),
			},
			env,
			"Has to accept correct path",
		)

		env, err = build_environment([]string{
			fmt.Sprintf("%s=/jenkins/kunde1", key, valid_path),
		})
		assert.NotEqual(t, nil, err, "Do return a error")
		assert.Equal(
			t,
			[]string{},
			env,
			"Error path has to be within the workspace",
		)

		env, err = build_environment([]string{
			fmt.Sprintf("%s=%s/kunde2/../../kunde1", key, valid_path),
		})
		assert.NotEqual(t, nil, err, "Do return a error")
		assert.Equal(
			t,
			[]string{},
			env,
			"Error path has to be within the workspace",
		)

		env, err = build_environment([]string{
			fmt.Sprintf("%s=jenkins/kunde1", key, valid_path),
		})
		assert.NotEqual(t, nil, err, "Do return a error")
		assert.Equal(
			t,
			[]string{},
			env,
			"Error path has to be absoulte",
		)
	}
}

func TestEnvironmentBuildSshAuthSock(t *testing.T) {
	key := "SSH_AUTH_SOCK"
	value := "/tmp/jenkins8342300640468645602.jnr"
	env, err := build_environment([]string{
		fmt.Sprintf("%s=%s", key, value),
	})
	assert.Equal(t, nil, err, "Do not return a error")
	assert.Equal(
		t,
		[]string{
			fmt.Sprintf("%s=%s", key, value),
		},
		env,
		"Error path has to be within the workspace",
	)
	assert.Equal(t, true, stringInSlice(value, config.files_to_copy), "Path is not copied to the container copy list")

	env, err = build_environment([]string{
		fmt.Sprintf("%s=relative/%s", key, value),
	})
	assert.NotEqual(t, nil, err, "Do not accept relative paths")
	assert.Equal(
		t,
		[]string{},
		env,
		"Error path must be removed frome env",
	)
}

func TestEnvironmentBuildBuildId(t *testing.T) {
	key := "BUILD_ID"
	value_i := 123
	env, err := build_environment([]string{
		fmt.Sprintf("%s=%d", key, value_i),
	})
	assert.Equal(t, nil, err, "Do not return a error")
	assert.Equal(
		t,
		[]string{
			fmt.Sprintf("%s=%d", key, value_i),
		},
		env,
		"Correct build_id has to be in env",
	)
	assert.Equal(t, value_i, config.build_id, "build_id is not copied to config")

	env, err = build_environment([]string{
		fmt.Sprintf("%s=abc", key),
	})
	assert.Equal(t, nil, err, "False error returned")
	assert.Equal(
		t,
		[]string{},
		env,
		"Incorrect build_id is in env",
	)
}

func TestEnvironmentBuildJobName(t *testing.T) {
	key := "JOB_NAME"
	value := "kunden-datev-magazin_-_wordpress_magazin_cs_temp"
	env, err := build_environment([]string{
		fmt.Sprintf("%s=%s", key, value),
	})
	assert.Equal(t, nil, err, "Do not return a error")
	assert.Equal(
		t,
		[]string{
			fmt.Sprintf("%s=%s", key, value),
		},
		env,
		"job_name has to be in env",
	)
	assert.Equal(t, value, config.job_name, "job_name is not copied to config")
}
