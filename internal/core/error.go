package core

import "fmt"

func Fail(err error) error {
	if err != nil {
		fmt.Printf("%v\n", err)
		panic(err)
	}
	return err
}
