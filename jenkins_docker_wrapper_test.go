package main

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

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
func TestEnvrionmentBuild(t *testing.T) {

	valid_path := "/jenkins/workspace"
	valid_user := "jenkins"
	config.jenkins_user = valid_user
	config.jenkins_workspace = valid_path

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

	env, err = build_environment([]string{
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

	// Validate workspace pwd directory
	for _, key := range []string{"PWD", "WORKSPACE"} {

		env, err = build_environment([]string{
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
