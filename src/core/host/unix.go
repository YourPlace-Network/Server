//go:build linux || freebsd || openbsd || aix

package host

import (
	"YourPlace/src/core"
	"YourPlace/src/core/security"
	_ "embed"
	"fmt"
	"log"
	"net"
	_os "os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

//go:embed bin/unix/yourplace.service
var systemdUnit []byte

//go:embed bin/unix/yourplace-logrotate
var yourplaceLogrotate []byte

//go:embed bin/ipfs/linux/ipfs
var ipfsBin []byte

var (
	listener *net.UDPConn
	mutex    sync.Mutex
)

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
	exec.Command("xdg-open", url).Start()
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
	RunShellCommand("pkill -9 " + processName)
	return !DoesProcExist(processName)
}
func ReleaseMutex() {
	mutex.Lock()
	defer mutex.Unlock()
	if listener != nil {
		_ = listener.Close()
		listener = nil
	}
}
func InstallFFMPEG() bool {
	return true
}
func InstallIPFS() bool {
	core.LogDebug("Installing IPFS binary")
	KillProcess("YourPlaceIpfs")

	// Check if embedded binary exists and has content
	if len(ipfsBin) == 0 {
		core.LogError("IPFS binary is not embedded in this build")
		return false
	}
	core.LogDebug("IPFS embedded binary size: " + fmt.Sprintf("%d bytes", len(ipfsBin)))

	if IsEmbeddedFileEqual(ipfsBin, GetInstallDir()+"YourPlaceIpfs") {
		core.LogDebug("IPFS binary already installed and matches embedded version")
		return true
	}

	ipfsRepo := GetDataDir() + ".ipfs"
	ipfsPath := "IPFS_PATH=" + ipfsRepo

	core.LogDebug("Writing IPFS binary to: " + GetInstallDir() + "YourPlaceIpfs")
	if !WriteEmbeddedBinary(ipfsBin, GetInstallDir()+"YourPlaceIpfs") {
		core.LogError("Failed to write IPFS binary")
		return false
	}

	core.LogDebug("Setting IPFS binary permissions")
	RunShellCommand("chmod 0755 " + GetInstallDir() + "YourPlaceIpfs")

	// Only run init if config doesn't exist
	if !DoesExist(ipfsRepo + PathSeparator + "config") {
		core.LogDebug("Running IPFS init")
		output := RunShellCommandEnv(GetInstallDir()+"YourPlaceIpfs init", []string{ipfsPath})
		core.LogDebug("IPFS init output: " + output)
	} else {
		core.LogDebug("IPFS already initialized")
	}

	return true
}
func InstallAutorun() bool {
	return true
}
func InstallHelper() bool {
	return true
}
func HelperCall(action string) (string, error) {
	return "", nil
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
	exec.Command("sh", "-c", security.SanitizeCommandInjection(command)).Start()
}
func RunShellCommandNoWaitEnv(command string, env []string) {
	cmd := exec.Command("sh", "-c", security.SanitizeCommandInjection(command))
	cmd.Env = env
	cmd.Start()
}
func DoesProcExist(name string) bool {
	cmd := exec.Command("pgrep", name)
	err := cmd.Run()
	return err == nil
}
func RunIPFS() bool {
	ipfsRepo := GetDataDir() + ".ipfs"
	ipfsPath := "IPFS_PATH=" + ipfsRepo
	DeleteIfExists(GetDataDir() + ".ipfs" + PathSeparator + "repo.lock")
	go RunShellCommandNoWaitEnv(GetInstallDir()+"YourPlaceIpfs daemon --migrate", []string{ipfsPath})
	return true
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
func IsAdmin() bool {
	return _os.Geteuid() == 0
}
func CreateMutex(name string) bool {
	mutex.Lock()
	defer mutex.Unlock()
	if listener != nil {
		return true
	}
	port := 54321
	if name == "YourPlaceHelper" {
		port = 54322
	}
	addr := &net.UDPAddr{
		Port: port,
		IP:   net.ParseIP("127.0.0.1"),
	}
	var err error
	listener, err = net.ListenUDP("udp", addr)
	if err != nil {
		core.LogError("Could not create mutex: " + err.Error())
		return false
	}
	c := make(chan _os.Signal, 1)
	signal.Notify(c, _os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		Shutdown(0)
	}()
	return true
}
func IsOnBattery() bool {
	cmd := exec.Command("upower", "-i", "/org/freedesktop/UPower/devices/battery_BAT0")
	output, err := cmd.Output()
	if err != nil {
		if _, statErr := _os.Stat("/sys/class/power_supply/BAT0/status"); statErr == nil {
			statusBytes, readErr := _os.ReadFile("/sys/class/power_supply/BAT0/status")
			if readErr == nil {
				status := strings.TrimSpace(string(statusBytes))
				return status == "Discharging"
			}
		}
		return false
	}
	outputStr := string(output)
	return strings.Contains(outputStr, "state:") && strings.Contains(outputStr, "discharging")
}
func RemoveScheduledTask(serviceName string) {
	RunShellCommand("sudo systemctl stop " + serviceName + ".service")
	RunShellCommand("sudo systemctl disable " + serviceName + ".service")
	RunShellCommand("sudo rm -f /etc/systemd/system/" + serviceName + ".service")
	RunShellCommand("sudo systemctl daemon-reload")
}
