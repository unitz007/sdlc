package models

import "errors"

type Task struct {
	Program string `json:"program"`
	Run     string `json:"run"`
	Test    string `json:"test"`
	Build   string `json:"build"`
}

func (c Task) Command(field string) (string, error) {
	switch field {
	case "run":
		return c.Run, nil
	case "test":
		return c.Test, nil
	case "build":
		return c.Build, nil
	default:
		return "", errors.New("invalid command")
	}
}
