package models

type Build struct {
	Builds []Command `json:"builds"`
}

func NewBuild(builds []Command) Build {
	return Build{Builds: builds}
}
