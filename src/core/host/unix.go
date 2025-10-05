//go:build linux || freebsd || openbsd || aix

package host

import (
	"YourPlace/src/core"
	_ "embed"
	"fmt"
	"log"
	_os "os"
	"os/exec"
	"path/filepath"
	"strings"
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
func CreateShortcut(port int) {

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
func HelperCall(action string) (string, error) {
	return "", nil // No helper on Linux
}
func RunShellCommand(command string) string {
	cmd := exec.Command("sh", "-c", command)
	output, _ := cmd.CombinedOutput()
	return string(output)
}
func RunShellCommandEnv(command string, env []string) string {
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = env
	output, _ := cmd.CombinedOutput()
	return string(output)
}
func RunShellCommandNoWait(command string) {
	exec.Command("sh", "-c", command).Start()
}
func RunShellCommandNoWaitEnv(command string, env []string) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = env
	cmd.Start()
}
func DoesProcExist(name string) bool {
	cmd := exec.Command("pgrep", name)
	err := cmd.Run()
	return err == nil
}
func RunIPFS() bool {
	// Assume IPFS is installed via package manager and available in PATH
	cmd := exec.Command("ipfs", "daemon", "--migrate")
	err := cmd.Start()
	return err == nil
}
func isSecretToolAvailable() bool {
	cmd := exec.Command("which", "secret-tool")
	err := cmd.Run()
	return err == nil
}
func AddSecret(name string, secret string) {
	if !isSecretToolAvailable() {
		core.LogWarn("secret-tool is not available on this system. Please install libsecret-tools package.")
		return
	}
	cmd := exec.Command("secret-tool", "store", "--label="+name, "application", "YourPlace", "name", name)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		core.LogError("Failed to create stdin pipe for secret storage: " + err.Error())
		return
	}
	if err := cmd.Start(); err != nil {
		core.LogError("Failed to start secret-tool for storage: " + err.Error())
		return
	}
	if _, err := stdin.Write([]byte(secret)); err != nil {
		core.LogError("Failed to write secret: " + err.Error())
		return
	}
	stdin.Close()
	if err := cmd.Wait(); err != nil {
		core.LogError("Failed to store secret: " + name)
		return
	}
}
func GetSecret(name string) string {
	if !isSecretToolAvailable() {
		core.LogWarn("secret-tool is not available on this system. Please install libsecret-tools package.")
		return ""
	}
	cmd := exec.Command("secret-tool", "lookup", "application", "YourPlace", "name", name)
	output, err := cmd.Output()
	if err != nil {
		core.LogError("Failed to retrieve secret: " + name)
		return ""
	}
	return strings.TrimSpace(string(output))
}
func DeleteSecret(name string) {
	if !isSecretToolAvailable() {
		core.LogWarn("secret-tool is not available on this system. Please install libsecret-tools package.")
		return
	}
	cmd := exec.Command("secret-tool", "clear", "application", "YourPlace", "name", name)
	if err := cmd.Run(); err != nil {
		core.LogError("Failed to delete secret: " + name + " - " + err.Error())
		return
	}
}
