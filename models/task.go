package models

type Task struct {
	Program string `json:"program"`
	Run     string `json:"run"`
	Test    string `json:"test"`
	Build   string `json:"build"`
}

func NewTask(program, run, test, build string) Task {
	return Task{
		Program: program,
		Test:    test,
		Build:   build,
		Run:     run,
	}
}
