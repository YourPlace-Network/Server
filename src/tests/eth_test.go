package tests

import (
	"YourPlace/src/core/security"
	"testing"
)

// https://blog.alexellis.io/golang-writing-unit-tests/

func TestEthAddressValidation(t *testing.T) {
	address := "0x695F8a4AB5979Ebb42273B52b8Ba6d6Ca0B4c4FC"
	if !security.IsValidEthAddress(address) {
		t.Error("Address is not valid")
		return
	}
	t.Log("Address is valid")
	return
}
