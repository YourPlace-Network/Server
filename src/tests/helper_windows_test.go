package tests

import (
	"YourPlace/src/core/host"
	"testing"
)

func TestHelperConnection(t *testing.T) {
	output, err := host.HelperCall("ping")
	if err != nil {
		t.Error(err)
	}
	t.Log(output)
}
