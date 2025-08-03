//go:build darwin

package host

import (
	"YourPlace/src/core"
	"YourPlace/src/core/security"
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
	"golang.org/x/sys/unix"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

type HelperRequest struct {
	Method string   `json:"method"`
	Params []string `json:"params"`
}
type HelperResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

//go:embed bin/helper/osx/helper.version
var helperVersion []byte

//go:embed bin/ipfs/osx/ipfs
var ipfsBinArm []byte

//go:embed bin/ipfs/osx/ipfs
var ipfsBin []byte

//go:embed bin/ffmpeg/osx/ffmpeg
var ffmpegBin []byte

var (
	listener *net.UDPConn
	mutex    sync.Mutex
)

const (
	PathSeparator     = string(os.PathSeparator)
	PathListSeparator = string(':')
	BinaryExtension   = ""
	HelperSocketAddr  = "/tmp/YourPlaceHelper.sock"
)

func GetServerID() string {
	cmd := exec.Command("sysctl", "-n", "machdep.cpu.brand_string", "machdep.cpu.features")
	var out bytes.Buffer
	err := cmd.Run()
	if err != nil {
		core.LogError("Could not generate Server ID: " + err.Error())
		return ""
	}
	return security.Hash(out.String())
}
func GetFreeDiskSpace(driveLetter string) uint64 {
	var stat unix.Statfs_t
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalln("could not get disk space: " + err.Error())
	}
	unix.Statfs(wd, &stat)
	return stat.Bavail * uint64(stat.Bsize)
}
func GetSelfFullPath() string {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalln(err)
	}
	return filepath.Dir(wd)
}
func Restart() {
	if HelperPing() {
		HelperRestart()
	} else {
		core.LogFatal("Could not restart YourPlace. IPC server not responding")
	}
}
func CreateShortcut(port int) {

}
func DoesProcExist(name string) bool {
	cmd := exec.Command("pgrep", name)
	err := cmd.Run()
	return err == nil
}
func CreateProcess(command string) {
	cmd := exec.Command(command)
	_, err := cmd.Output()
	if err != nil {
		core.LogError("Could not start process: " + err.Error())
		return
	}
}
func KillProcess(processName string) bool {
	RunShellCommand("pkill -9 -f " + processName)
	return true
}
func IsAdmin() bool {
	return os.Geteuid() == 0
}
func RemoveScheduledTask(helperServiceName string) {
	core.LogDebug("host.RemoveScheduledTask mac")
}
func InstallAutorun() bool {
	// Installed as part of the osx_packager.sh script
	return true
}
func CreateMutex(name string) bool {
	mutex.Lock()
	defer mutex.Unlock()
	// If we already have a listener, we're already running as the singleton
	if listener != nil {
		return true
	}
	// Determine the port based on application name
	port := 54321
	if name == "YourPlaceHelper" {
		port = 54322
	}
	// Bind the to the listening port
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
	// Setup cleanup for proper socket release
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		Shutdown(0)
	}()
	return true
}
func ReleaseMutex() {
	mutex.Lock()
	defer mutex.Unlock()
	if listener != nil {
		_ = listener.Close()
		listener = nil
	}
}
func CreateUninstaller() {}
func EscapePath(path string) string {
	escaped := strings.ReplaceAll(path, "/", "//")
	return escaped
}
func OpenBrowser(url string) {
	if !security.IsValidURL(url) {
		return
	}
	err := exec.Command("open", url).Start()
	if err != nil {
		return
	}
}
func RunShellCommand(command string) string {
	return RunShellCommandEnv(command, []string{})
}
func RunShellCommandEnv(command string, env []string) string {
	_command := security.SanitizeCommandInjection(command)
	cmd := exec.Command("bash", "-c", _command)
	cmd.Env = os.Environ()
	for _, variable := range env {
		sanitizedVariable := security.SanitizeCommandInjection(variable)
		cmd.Env = append(cmd.Env, sanitizedVariable)
	}
	cmd.Dir, _ = os.Getwd()
	//cmd.Stdout = os.Stdout // debug
	var out bytes.Buffer
	cmd.Stdout = &out
	//cmd.Stderr = os.Stderr // debug
	err := cmd.Run()
	if err != nil {
		return ""
	} else {
		return out.String()
	}
}
func RunShellCommandNoWait(command string) {
	RunShellCommandNoWaitEnv(command, []string{})
}
func RunShellCommandNoWaitEnv(command string, env []string) {
	//core.LogInfo("Running shell command: " + command + "\nWith ENV: " + strings.Join(env, "\n")) // debug
	_command := security.SanitizeCommandInjection(command)
	cmd := exec.Command("bash", "-c", _command)
	cmd.Env = os.Environ()
	for _, variable := range env {
		sanitizedVariable := security.SanitizeCommandInjection(variable)
		cmd.Env = append(cmd.Env, sanitizedVariable)
	}
	cmd.Dir, _ = os.Getwd()
	//cmd.Stdout = os.Stdout // debug
	//cmd.Stderr = os.Stderr // debug
	err := cmd.Start()
	if err != nil {
		core.LogWarn("Could not start command: " + _command + "\n" + err.Error())
	}
}
func GetKeyboardLayout() string {
	cmd := exec.Command("defaults", "read", "-g", "AppleKeyboardLayout")
	output, err := cmd.Output()
	if err != nil {
		core.LogError("Could not get OSX keyboard layout: " + err.Error())
		return ""
	}
	layout := strings.TrimSpace(string(output))
	return layout
}
func IsOnBattery() bool {
	cmd := exec.Command("pmset", "-g", "ps")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return false
	}
	output := out.String()
	return !strings.Contains(output, "'AC Power'")
}
func AddSecret(name string, secret string) {
	currentUser, _ := user.Current()
	cmd := exec.Command("security", "add-generic-password",
		"-s", security.SanitizePathTraversal(name),
		"-a", currentUser.Username,
		"-w", security.SanitizePathTraversal(secret))
	output, err := cmd.CombinedOutput()
	if err != nil {
		core.LogError("Failed to save secret: " + name)
		return
	}
	core.LogDebug(string(output))
}
func GetSecret(name string) string {
	currentUser, _ := user.Current()
	cmd := exec.Command("security", "find-generic-password",
		"-s", security.SanitizePathTraversal(name),
		"-a", currentUser.Username,
		"-w")
	output, err := cmd.CombinedOutput()
	if err != nil {
		core.LogError("Failed to retrieve secret: " + name)
		return ""
	}
	return strings.TrimSpace(string(output))
}

/* ------ OS Specific Business Logic ------ */
func InstallIPFS() bool {
	KillProcess("YourPlaceIpfs")
	if IsEmbeddedFileEqual(ipfsBin, GetInstallDir()+"YourPlaceIpfs") {
		return true
	}
	if GetCPUArch() == 64 {
		ipfsRepo := GetDataDir() + ".ipfs"
		ipfsPath := "IPFS_PATH=" + EscapePath(ipfsRepo)
		if GetCPUVendor() == "intel" {
			WriteEmbeddedBinary(ipfsBin, GetInstallDir()+"YourPlaceIpfs")
		} else if GetCPUVendor() == "arm" {
			WriteEmbeddedBinary(ipfsBinArm, GetInstallDir()+"YourPlaceIpfs")
		} else {
			core.LogError("Could not recognize CPU Vendor")
			return false
		}
		RunShellCommand("chmod 0744 " + GetInstallDir() + "YourPlaceIpfs")
		RunShellCommandEnv(GetInstallDir()+"YourPlaceIpfs init", []string{ipfsPath})
		return true
	} else {
		return false
	}
}
func RunIPFS() bool {
	ipfsRepo := GetDataDir() + ".ipfs"
	ipfsPath := "IPFS_PATH=" + EscapePath(ipfsRepo)
	DeleteIfExists(GetDataDir() + ".ipfs" + PathSeparator + "repo.lock")
	go RunShellCommandNoWaitEnv(GetInstallDir()+"YourPlaceIpfs daemon --migrate", []string{ipfsPath})
	return true
}
func InstallFFMPEG() bool {
	if GetCPUArch() == 64 {
		ffmpegPath := GetInstallDir() + "YourPlaceFfmpeg"
		WriteEmbeddedBinary(ffmpegBin, ffmpegPath)
		RunShellCommand("chmod 0744 " + ffmpegPath)
		return true
	} else {
		core.LogError("Could not recognize CPU Arch for FFMPEG installation")
		return false
	}
}
func GetFfmpegBin() string {
	ffmpeg := GetInstallDir() + "YourPlaceFfmpeg"
	return ffmpeg
}
func InstallFarcaster() bool {
	// https://docs.farcaster.xyz/hubble/install
	return false
}
func InstallOllama() bool { // todo
	/*if !IsBigSurOrLater() {
		core.LogError("Ollama is only supported on macOS Big Sur or later")
		return false
	}
	url := "https://ollama.com/download/Ollama-darwin.zip"
	err := network.HttpGetFile(url, GetInstallDir()+"ollama")
	if err != nil {
		core.LogError("Could not download Ollama: " + err.Error())
		return false
	}
	RunShellCommand("chmod 0744 " + GetInstallDir() + "ollama")*/
	return false
}
func InstallGethNode() bool {
	return false
}
func InstallRunBaseNode() bool {
	return false
}
func RunGethNode() bool {
	return false
}
func IsDockerSocketExist() bool {
	dockerSocketPath := "/var/run/docker.sock"
	if _, err := os.Stat(dockerSocketPath); os.IsNotExist(err) {
		return false
	}
	conn, err := net.Dial("unix", dockerSocketPath)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}
func IsBigSurOrLater() bool {
	/*var osVersion [256]byte
	size := uintprt(len(osVersion))
	syscall.Syscall(syscall.SYS_SYSCTL,
		uintptr(unsafe.Pointer(syscall.StringBytePtr("kern.osrelease"))),
		uintptr(unsafe.Pointer(&osVersion[0])),
		uintptr(unsafe.Pointer(&size)),
		0,
		0)
	versionStr := string(osVersion[:size])
	var majorVersion int
	fmt.Sscanf(versionStr, "%d", &majorVersion)
	if majorVersion >= 20 {
		return true
	}*/
	return false
}

/* ------ Helper API ------ */
func InstallHelper() bool {
	// check helper version
	installedHelperVersionBytes, _ := os.ReadFile(GetInstallDir() + "helper.version")
	needsUpdate := false
	if !bytes.Equal(installedHelperVersionBytes, helperVersion) {
		needsUpdate = true
	}
	if !needsUpdate {
		core.LogDebug("Helper is already up to date")
		return true
	}

	// Write version file
	err := os.WriteFile(GetInstallDir()+"helper.version", helperVersion, 0644)
	if err != nil {
		core.LogError("Could not write helper version file: " + err.Error())
		return false
	}
	return true
}
func HelperIsInstalled() bool {
	if !DoesExist(GetInstallDir() + "YourPlaceHelper" + BinaryExtension) {
		core.LogWarn("YourPlaceHelper binary does not exist")
		return false
	}
	if !DoesExist(GetInstallDir() + "helper.version") {
		core.LogWarn("Helper version file does not exist")
		return false
	}
	if !DoesProcExist("YourPlaceHelper" + BinaryExtension) {
		core.LogWarn("YourPlaceHelper process does not exist")
		return false
	}
	return true
}
func HelperCall(method string) (string, error) {
	conn, err := net.Dial("unix", HelperSocketAddr)
	if err != nil {
		return "failure", core.LogErrorReturn("Could not connect to helper socket: " + err.Error())
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	request := HelperRequest{
		Method: method,
	}
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return "failure", core.LogErrorReturn("Could not encode helper request: " + err.Error())
	}
	requestJSON = append(requestJSON, '\n')
	_, err = conn.Write(requestJSON)
	if err != nil {
		return "failure", core.LogErrorReturn("Could not send helper request: " + err.Error())
	}
	responseReader := bufio.NewReader(conn)
	responseLine, err := responseReader.ReadString('\n')
	if err != nil {
		return "failure", core.LogErrorReturn("Could not read helper response: " + err.Error())
	}
	var response HelperResponse
	err = json.Unmarshal([]byte(responseLine), &response)
	if err != nil {
		return "failure", core.LogErrorReturn("Could not decode helper response: " + err.Error())
	}
	if response.Status != "success" {
		core.LogDebug("Helper IPC didn't return success: " + response.Message)
		return "failure", core.LogErrorReturn("Helper response status is not success: " + response.Message)
	} else {
		core.LogDebug("Helper IPC returned success")
		return response.Message, nil
	}
}
func HelperRestart() {
	status, err := HelperCall("restart")
	if err != nil {
		core.LogError("Could not restart helper 1: " + err.Error())
		return
	}
	if status == "success" {
		core.LogInfo("YourPlace Restarting")
		Shutdown(0)
	} else {
		core.LogError("Could not restart helper 2: " + err.Error())
	}
}
func HelperPing() bool {
	for i := 0; i <= 5; i++ {
		status, err := HelperCall("ping")
		if err != nil {
			core.LogError("Could not ping helper 1: " + err.Error())
		}
		if status == "success" {
			core.LogDebug("Helper Ping Success: " + status)
			return true
		} else {
			core.LogError("Could not ping helper 2: " + status)
		}
	}
	core.LogError("Could not ping helper 3: too may attempts")
	return false
}
func HelperWhitelistTor() bool {
	status, err := HelperCall("whitelist_tor")
	if err != nil {
		core.LogError("Could not whitelist tor binary: " + err.Error())
		return false
	}
	if status == "success" {
		core.LogInfo("Tor binary whitelisted successfully")
		return true
	} else {
		core.LogError("Could not whitelist tor binary: " + status)
		return false
	}
}
