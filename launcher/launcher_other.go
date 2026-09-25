//go:build !darwin

package launcher

func HandleCommand(protocol, domain string, port int) bool {
	return false
}
func Start(gateway bool) {
}
