package tests

import (
	"regexp"
	"testing"
)

// https://blog.alexellis.io/golang-writing-unit-tests/

func TestHelloName(t *testing.T) {
	name := "Gladys"
	want := regexp.MustCompile(`\b` + name + `\b`)
	msg := string("Gladys")
	if !want.MatchString(msg) {
		t.Fatalf(`Hello("Gladys") = %q, want match for %#q, nil`, msg, want)
	}
}
