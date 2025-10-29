package core

import (
	"fmt"
	"os"
	"strings"
)

func MustNotBeEmptyEither(desc string, inputs ...string) {
	numEmpty := 0

	for i := range inputs {
		if strings.TrimSpace(inputs[i]) == "" {
			numEmpty++
		}
	}

	if len(inputs) == numEmpty {
		_ = Fail(fmt.Errorf(desc))
	}
}

func MustNotBeEmpty(desc string, input string) {
	if strings.TrimSpace(input) == "" {
		_ = Fail(fmt.Errorf(desc))
	}
}

func MustCreateFile(filename string) *os.File {
	f, err := os.Create(filename)
	if err != nil {
		_ = Fail(fmt.Errorf("error creating %s: %v", filename, err))
	}

	return f
}
