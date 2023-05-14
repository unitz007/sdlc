package models

import "testing"

var commandMock = NewCommand(
	"go.mod",
	taskMock,
)

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

func Test_fieldAssertions(t *testing.T) {
	commandTestTable := []struct {
		testCase  string
		testValue any
		expected  any
		message   string
	}{
		{
			"Build file field test",
			commandMock.BuildFile(),
			"go.mod",
			"buildFile field should be go.mod",
		},
		{
			"Task field test",
			commandMock.Task(),
			taskMock,
			"buildFile field should be go.mod",
		},
	}

	for _, testValue := range commandTestTable {
		if testValue.testValue != testValue.expected {
			t.Error(testValue.message)
		}
	}
}
