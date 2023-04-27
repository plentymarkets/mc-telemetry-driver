package helloworld

import "fmt"

// Greet says hello to someone
func Greet(name string) string {
	return fmt.Sprintf("Hello %s", name)
}
