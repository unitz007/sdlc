package models

type Commands interface {
	Task() *Task
}

type Command struct {
	Bf  string `json:"build_file"`
	Tsk *Task  `json:"task"`
}

func (c Command) BuildFile() string {
	return c.Bf
}

func (c Command) Task() *Task {
	return c.Tsk
}
