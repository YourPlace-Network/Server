package security

import (
	"YourPlace/src/core"
	"github.com/danieljoos/wincred"
	"runtime"
)

func AddSecret(name string, secret string) {
	// Store a secret
	if runtime.GOOS == "windows" {
		cred := wincred.NewGenericCredential(name)
		cred.CredentialBlob = []byte(secret)
		err := cred.Write()
		if err != nil {
			core.LogError("Failed to store secret: " + name)
		}
	}
}
func GetSecret(name string) string {
	if runtime.GOOS == "windows" {
		cred, err := wincred.GetGenericCredential(name)
		if err != nil {
			core.LogError("Failed to retrieve secret: " + name)
			return ""
		}
		return string(cred.CredentialBlob)
	}
	return ""
}
func DeleteSecret(name string) {
	if runtime.GOOS == "windows" {
		credential, _ := wincred.GetGenericCredential(name)
		err := credential.Delete()
		if err != nil {
			core.LogError("Failed to delete secret: " + name)
		}
	}
}
