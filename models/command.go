package models

type command struct {
	buildFile string
	task      TaskSpec
}

type CommandSpec interface {
	BuildFile() string
	Task() TaskSpec
}

func NewCommand(buildFile string, task TaskSpec) CommandSpec {
	return command{
		buildFile: buildFile,
		task:      task,
	}
}

func (c command) BuildFile() string {
	return c.buildFile
}

func (c command) Task() TaskSpec {
	return c.task
}
