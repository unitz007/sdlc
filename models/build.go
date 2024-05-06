package models

type Build struct {
	Builds []Command `json:"builds"`
}

type Build2 struct {
	Build map[string]Task `json:""`
}
