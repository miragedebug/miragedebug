package shell

import (
	"context"
	"testing"
	"time"
)

func TestShellExecute(t *testing.T) {
	cases := []struct {
		name         string
		timeout      time.Duration
		commands     []string
		exceptOut    []byte
		exceptErrOut []byte
		exceptErr    string
	}{
		{
			name:    "oll true",
			timeout: time.Second * 10,
			commands: []string{
				"echo 1",
				"true",
			},
			exceptOut:    []byte("1\n"),
			exceptErrOut: []byte{},
			exceptErr:    "",
		},
		{
			name:    "timeout",
			timeout: time.Millisecond,
			commands: []string{
				"sleep 1",
			},
			exceptOut:    []byte{},
			exceptErrOut: []byte{},
			exceptErr:    "signal: killed",
		},
		{
			name:    "error",
			timeout: time.Second * 10,
			commands: []string{
				"echo 1",
				"false",
				"echo 2",
			},
			exceptOut:    []byte("1\n"),
			exceptErrOut: []byte{},
			exceptErr:    "exit status 1",
		},
		{
			name:    "error out",
			timeout: time.Second * 10,
			commands: []string{
				"echo 1 > /dev/stderr",
				"false",
				"echo 2",
			},
			exceptOut:    []byte(""),
			exceptErrOut: []byte("1\n"),
			exceptErr:    "exit status 1",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
			defer cancel()
			out, errOut, err := ExecuteCommands(ctx, c.commands)
			if err != nil {
				if c.exceptErr == "" {
					t.Errorf("except no error, but got %s", err)
				} else if err.Error() != c.exceptErr {
					t.Errorf("except error %s, but got %s", c.exceptErr, err)
				}
			} else if c.exceptErr != "" {
				t.Errorf("except error %s, but got no error", c.exceptErr)
			}
			if string(out) != string(c.exceptOut) {
				t.Errorf("except out %s, but got %s", string(c.exceptOut), string(out))
			}
			if string(errOut) != string(c.exceptErrOut) {
				t.Errorf("except errOut %s, but got %s", string(c.exceptErrOut), string(errOut))
			}
		})
	}
}
