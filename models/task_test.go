package models

import "testing"

var taskMock = NewTask(
	"go",
	"run .",
	"test .",
	"build",
)

func Test_taskValueAssertions(t *testing.T) {

	testTable := []struct {
		testCase  string
		testValue string
		expected  string
		message   string
	}{
		{
			"programTest",
			taskMock.Program,
			"go",
			"go should be equal to go",
		},
		{
			"runTest",
			taskMock.Run,
			"run .",
			"\"run .\" should be equal to \"run .\"",
		},
		{
			"testTest",
			taskMock.Test,
			"test .",
			"\"test .\" should be equal to \"test .\"",
		},
		{
			"buildTest",
			taskMock.Build,
			"build",
			"build should be equal to build",
		},
	}

	for _, testCase := range testTable {
		if testCase.testValue != testCase.expected {
			t.Error(testCase.message)
		}
	}
}

//func Test_taskShouldBeOfTypeTaskSpec(t *testing.T) {
//
//	test := func() bool {
//		switch taskMock.(type) {
//		case TaskSpec:
//			return true
//		default:
//			return false
//		}
//	}()
//
//	if !test {
//		t.Error("task is not of type TaskSpec")
//	}
//}
