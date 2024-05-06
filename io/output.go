package io

import (
	"fmt"
	"os"
)

func FatalPrint(v string) {
	fmt.Println("Error:", v)
	os.Exit(1)
}

func Print(v string) {
	fmt.Println("SDLC:", v)
}
