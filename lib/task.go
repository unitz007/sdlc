package lib

import "errors"

type Task struct {
<<<<<<< HEAD
	Run   string `json:"run"`
	Test  string `json:"test"`
	Build string `json:"build"`
=======
	Program string `json:"program"`
	Run     string `json:"run"`
	Test    string `json:"test"`
	Build   string `json:"build"`
>>>>>>> 87aea0d610d6ff539760d8551df3c39ca9bdb1f8
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
