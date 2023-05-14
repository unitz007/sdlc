package models

type task struct {
	program string
	run     string
	test    string
	build   string
}

type TaskSpec interface {
	Program() string
	Run() string
	Test() string
	Build() string
}

func newTask(program, run, test, build string) TaskSpec {
	return task{
		program: program,
		test:    test,
		build:   build,
		run:     run,
	}
}

func (t task) Program() string { return t.program }
func (t task) Test() string    { return t.test }
func (t task) Build() string   { return t.build }
func (t task) Run() string     { return t.run }
