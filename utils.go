package pddlocks

import (
	"os"
	"testing"
)

func SkipAutoTest(t *testing.T) {
	if os.Getenv("AUTO_TEST") == "true" {
		t.Skip("Skip unit testing in AUTO_TEST mode")
	}
}
