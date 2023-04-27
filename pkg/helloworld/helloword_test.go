package helloworld

import "testing"

func TestGreet(t *testing.T) {
	greeting := Greet("World")
	expected := "Hello World"
	if greeting != expected {
		t.Errorf("expected %s but got %s", expected, greeting)
	}
}
