package main

import (
	"fmt"
	"testing"
)

//func Test_readConfigData(t *testing.T) {
//
//	_, err := readConfigData(configData)
//
//	if err == nil {
//		t.Errorf("error: Could not load sdlc configuration")
//	}
//}

func Test_loadConfig(t *testing.T) {

	testCases := []struct {
		name      string
		testValue ConfigFile
	}{
		{
			name: "Empty String",
			testValue: ConfigFile{
				data: "",
			},
		},
		{
			name: "Non json String",
			testValue: ConfigFile{
				data: "normal test string",
			},
		},
	}

	for _, testCase := range testCases {
		task, err := loadConfig(testCase.testValue.data)

		if err == nil {
			t.Errorf("%s: config content should be in json format", testCase.name)
		}

		fmt.Println(task)
	}
}
