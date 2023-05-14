package models

var command command

func init() {

	command = command {
		buildFile: "go.mod",
		task: task {
			program: "run",
			run: "run",
			build: "build",
			test: "test",
		},
	}

}
