package models

type Command struct {
	BuildFile string `json:"build_file"`
	Task      Task   `json:"task"`
}

func NewCommand(buildFile string, task Task) Command {
	return Command{
		BuildFile: buildFile,
		Task:      task,
	}
}
