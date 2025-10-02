//go:build linux || freebsd || openbsd || aix

package host

import (
	"fmt"
	"log"
	_os "os"
	"os/exec"
	"path/filepath"
)

//go:embed bin/unix/yourplace.service
var systemdUnit []byte

//go:embed bin/unix/yourplace-logrotate
var yourplaceLogrotate []byte

const (
	PathSeparator     = string('/')
	PathListSeparator = string(':')
	BinaryExtension   = ""
)

func GetFreeDiskSpace(driveLetter string) uint64 {
	fmt.Println("linux")
	return 0
}
func GetSelfFullPath() string {
	wd, err := _os.Getwd()
	if err != nil {
		log.Fatalln(err)
	}
	return filepath.Dir(wd)
}
func Restart() {
	RunShellCommand("sudo systemctl restart yourplace.service")
}
func InstallRunBaseNode() bool {
	return false
}
func OpenBrowser(url string) {
	exec.Command("xdg-open", url).Start() // linux
}
func CreateAutoStart() {
	WriteEmbeddedBinary(systemdUnit, "/etc/systemd/system/yourplace.service")
	WriteEmbeddedBinary(yourplaceLogrotate, "/etc/logrotate.d/yourplace-logrotate")
	RunShellCommand("systemctl daemon-reload")
	RunShellCommand("systemctl enable yourplace.service")
	RunShellCommand("systemctl start yourplace.service")
}
func CreateShortcut() {

}
func ExecExtension() string {
	return ""
}
func HardenApp() {
	//RunShellCommand("sudo chown root:root " + GetSelfFullPath() + "/yourplace")
	//RunShellCommand("sudo chmod 755 " + GetSelfFullPath() + "/yourplace")
	//RunShellCommand("sudo setcap 'cap_net_bind_service=+ep' " + GetSelfFullPath() + "/yourplace")
}
func GetKeyboardLayout() string {
	return "Unknown - Unix"
}

func IsDockerSocketExist() bool {
	dockerSocketPath := "/var/run/docker.sock"
	if _, err := _os.Stat(dockerSocketPath); _os.IsNotExist(err) {
		return false
	}
	return true
}

// Stub functions for Linux (not needed for snapshot service)
func KillProcess(processName string) bool {
	return false
}

func ReleaseMutex() {
	// No-op on Linux
}

func InstallFFMPEG() bool {
	return true // Assume installed via package manager
}

func InstallIPFS() bool {
	return true // Assume installed via package manager
}

func InstallAutorun() bool {
	return true // Handled by systemd
}

func InstallHelper() bool {
	return true // No helper needed on Linux
}
