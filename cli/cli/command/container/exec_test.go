package container

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/docker/cli/cli"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/internal/test"
	"github.com/docker/cli/opts"
	"github.com/docker/docker/api/types"
	"github.com/pkg/errors"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/fs"
)

func withDefaultOpts(options execOptions) execOptions {
	options.env = opts.NewListOpts(opts.ValidateEnv)
	options.envFile = opts.NewListOpts(nil)
	if len(options.command) == 0 {
		options.command = []string{"command"}
	}
	return options
}

func TestParseExec(t *testing.T) {
	content := `ONE=1
TWO=2
	`

	tmpFile := fs.NewFile(t, t.Name(), fs.WithContent(content))
	defer tmpFile.Remove()

	testcases := []struct {
		options    execOptions
		configFile configfile.ConfigFile
		expected   types.ExecConfig
	}{
		{
			expected: types.ExecConfig{
				Cmd:          []string{"command"},
				AttachStdout: true,
				AttachStderr: true,
			},
			options: withDefaultOpts(execOptions{}),
		},
		{
			expected: types.ExecConfig{
				Cmd:          []string{"command1", "command2"},
				AttachStdout: true,
				AttachStderr: true,
			},
			options: withDefaultOpts(execOptions{
				command: []string{"command1", "command2"},
			}),
		},
		{
			options: withDefaultOpts(execOptions{
				interactive: true,
				tty:         true,
				user:        "uid",
			}),
			expected: types.ExecConfig{
				User:         "uid",
				AttachStdin:  true,
				AttachStdout: true,
				AttachStderr: true,
				Tty:          true,
				Cmd:          []string{"command"},
			},
		},
		{
			options: withDefaultOpts(execOptions{detach: true}),
			expected: types.ExecConfig{
				Detach: true,
				Cmd:    []string{"command"},
			},
		},
		{
			options: withDefaultOpts(execOptions{
				tty:         true,
				interactive: true,
				detach:      true,
			}),
			expected: types.ExecConfig{
				Detach: true,
				Tty:    true,
				Cmd:    []string{"command"},
			},
		},
		{
			options:    withDefaultOpts(execOptions{detach: true}),
			configFile: configfile.ConfigFile{DetachKeys: "de"},
			expected: types.ExecConfig{
				Cmd:        []string{"command"},
				DetachKeys: "de",
				Detach:     true,
			},
		},
		{
			options: withDefaultOpts(execOptions{
				detach:     true,
				detachKeys: "ab",
			}),
			configFile: configfile.ConfigFile{DetachKeys: "de"},
			expected: types.ExecConfig{
				Cmd:        []string{"command"},
				DetachKeys: "ab",
				Detach:     true,
			},
		},
		{
			expected: types.ExecConfig{
				Cmd:          []string{"command"},
				AttachStdout: true,
				AttachStderr: true,
				Env:          []string{"ONE=1", "TWO=2"},
			},
			options: func() execOptions {
				o := withDefaultOpts(execOptions{})
				o.envFile.Set(tmpFile.Path())
				return o
			}(),
		},
		{
			expected: types.ExecConfig{
				Cmd:          []string{"command"},
				AttachStdout: true,
				AttachStderr: true,
				Env:          []string{"ONE=1", "TWO=2", "ONE=override"},
			},
			options: func() execOptions {
				o := withDefaultOpts(execOptions{})
				o.envFile.Set(tmpFile.Path())
				o.env.Set("ONE=override")
				return o
			}(),
		},
	}

	for _, testcase := range testcases {
		execConfig, err := parseExec(testcase.options, &testcase.configFile)
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(testcase.expected, *execConfig))
	}
}

func TestParseExecNoSuchFile(t *testing.T) {
	execOpts := withDefaultOpts(execOptions{})
	execOpts.envFile.Set("no-such-env-file")
	execConfig, err := parseExec(execOpts, &configfile.ConfigFile{})
	assert.ErrorContains(t, err, "no-such-env-file")
	assert.Check(t, os.IsNotExist(err))
	assert.Check(t, execConfig == nil)
}

func TestRunExec(t *testing.T) {
	var testcases = []struct {
		doc           string
		options       execOptions
		client        fakeClient
		expectedError string
		expectedOut   string
		expectedErr   string
	}{
		{
			doc: "successful detach",
			options: withDefaultOpts(execOptions{
				container: "thecontainer",
				detach:    true,
			}),
			client: fakeClient{execCreateFunc: execCreateWithID},
		},
		{
			doc:     "inspect error",
			options: newExecOptions(),
			client: fakeClient{
				inspectFunc: func(string) (types.ContainerJSON, error) {
					return types.ContainerJSON{}, errors.New("failed inspect")
				},
			},
			expectedError: "failed inspect",
		},
		{
			doc:           "missing exec ID",
			options:       newExecOptions(),
			expectedError: "exec ID empty",
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.doc, func(t *testing.T) {
			cli := test.NewFakeCli(&testcase.client)

			err := runExec(cli, testcase.options)
			if testcase.expectedError != "" {
				assert.ErrorContains(t, err, testcase.expectedError)
			} else {
				if !assert.Check(t, err) {
					return
				}
			}
			assert.Check(t, is.Equal(testcase.expectedOut, cli.OutBuffer().String()))
			assert.Check(t, is.Equal(testcase.expectedErr, cli.ErrBuffer().String()))
		})
	}
}

func execCreateWithID(_ string, _ types.ExecConfig) (types.IDResponse, error) {
	return types.IDResponse{ID: "execid"}, nil
}

func TestGetExecExitStatus(t *testing.T) {
	execID := "the exec id"
	expecatedErr := errors.New("unexpected error")

	testcases := []struct {
		inspectError  error
		exitCode      int
		expectedError error
	}{
		{
			inspectError: nil,
			exitCode:     0,
		},
		{
			inspectError:  expecatedErr,
			expectedError: expecatedErr,
		},
		{
			exitCode:      15,
			expectedError: cli.StatusError{StatusCode: 15},
		},
	}

	for _, testcase := range testcases {
		client := &fakeClient{
			execInspectFunc: func(id string) (types.ContainerExecInspect, error) {
				assert.Check(t, is.Equal(execID, id))
				return types.ContainerExecInspect{ExitCode: testcase.exitCode}, testcase.inspectError
			},
		}
		err := getExecExitStatus(context.Background(), client, execID)
		assert.Check(t, is.Equal(testcase.expectedError, err))
	}
}

func TestNewExecCommandErrors(t *testing.T) {
	testCases := []struct {
		name                 string
		args                 []string
		expectedError        string
		containerInspectFunc func(img string) (types.ContainerJSON, error)
	}{
		{
			name:          "client-error",
			args:          []string{"5cb5bb5e4a3b", "-t", "-i", "bash"},
			expectedError: "something went wrong",
			containerInspectFunc: func(containerID string) (types.ContainerJSON, error) {
				return types.ContainerJSON{}, errors.Errorf("something went wrong")
			},
		},
	}
	for _, tc := range testCases {
		cli := test.NewFakeCli(&fakeClient{inspectFunc: tc.containerInspectFunc})
		cmd := NewExecCommand(cli)
		cmd.SetOut(io.Discard)
		cmd.SetArgs(tc.args)
		assert.ErrorContains(t, cmd.Execute(), tc.expectedError)
	}
}
