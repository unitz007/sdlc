package models

import "testing"

var commandMock = NewCommand(
	"go.mod",
	task{
		program: "run",
		run:     "run",
		build:   "build",
		test:    "test",
	})

func Test_commandStructShouldImplementCommandSpec(t *testing.T) {

	test := func() bool {
		switch commandMock.(type) {
		case CommandSpec:
			return true
		default:
			return false
		}
	}()

	if !test {
		t.Error("command struct does not implement CommandSpec")
	}
}
