//go:build go1.24 && windows

package main

import (
	"YourPlace/src/core"
	"YourPlace/src/core/host"
	"YourPlace/src/core/network"
	"YourPlace/src/core/security"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/robfig/cron/v3"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

//go:embed resources/helper.manifest
var manifest []byte // embed Windows manifest

//go:embed resources/uninstall.ps1
var uninstallScriptBytes []byte // embed uninstall script

type HelperAction struct {
	Type string `json:"type"`
}

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	getConsoleWindow = kernel32.NewProc("GetConsoleWindow")
	showWindow       = user32.NewProc("ShowWindow")
	messageBox       = user32.NewProc("MessageBoxW")
	logger           *log.Logger
	loggerMutex      sync.Mutex
)

const (
	version     = "0.0.7"
	serviceName = "YourPlaceHelper"
	pipeName    = `\\.\pipe\yourplacehelper`
	colorRed    = "\033[1;31m"
	colorYellow = "\033[1;33m"
	colorBlue   = "\033[1;34m"
	colorPurple = "\033[1;35m"
	colorNone   = "\033[0m"
)

func main() {
	// Initialize the Windows Service
	hwnd, _, _ := getConsoleWindow.Call() // Hide the console window
	if hwnd != 0 {
		showWindow.Call(hwnd, syscall.SW_HIDE)
	}
	if !host.CreateMutex(serviceName) { // Singleton pattern
		fmt.Println("Another instance of the helper service is already running")
		os.Exit(0)
	}
	defer host.ReleaseMutex()
	if !host.IsAdmin() { // Ensure running as administrator
		fmt.Println("Helper service must be run as administrator")
		os.Exit(0)
	}
	_ = LogInit("yourplacehelper") // Initialize the logger
	LogInfo("Starting YourPlace Helper")
	_ = os.WriteFile(host.GetInstallDir()+"helper.version", []byte(version), 0744) // write the version string to file

	// Check for command line flags
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			LogInfo("Install YourPlace Helper Flag")
			install()
			os.Exit(0)
		case "uninstall":
			LogInfo("Uninstall YourPlace Complete Flag")
			uninstall(false, false)
			os.Exit(0)
		case "uninstall -keepUpload":
			LogInfo("Uninstall YourPlace Keep Uploads")
			uninstall(true, false)
			os.Exit(0)
		case "uninstall -keepBlockchain":
			LogInfo("Uninstall YourPlace Keep Blockchain")
			uninstall(false, true)
			os.Exit(0)
		case "uninstall -keepUpload -keepBlockchain":
			LogInfo("Uninstall YourPlace Keep Uploads and Blockchain")
			uninstall(true, true)
			os.Exit(0)
		case "version":
			LogInfo("YourPlace Helper Version: " + version)
			os.Exit(0)
		case "restart":
			LogInfo("Restart YourPlace Server Flag")
			restart()
			os.Exit(0)
		default:
			os.Exit(0)
		}
	}

	c := cron.New(cron.WithSeconds())
	c.AddFunc("@every 5m", func() { // ETH price updater
		execPath := host.GetInstallDir() + "YourPlace" + host.BinaryExtension
		err := host.RunAsUser(execPath)
		if err != nil {
			LogError("Failed to start YourPlace: " + err.Error())
		}
	})

	taskInstalled, err := host.IsScheduledTaskInstalled(serviceName)
	if !taskInstalled || err != nil {
		LogWarn("Scheduled task not installed, installing")
		host.InstallScheduledTask(serviceName) // Install scheduled task to start the service if not already running
	}
	host.StartScheduledTask(serviceName)
	startIPCServer()
}

func startIPCServer() {
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GRGW;;;AU)") // Create a security descriptor that allows everyone to connect
	if err != nil {
		LogError("Failed to create security descriptor: " + err.Error())
		return
	}
	sa := &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd, InheritHandle: 1}
	for {
		pipeHandle, err := windows.CreateNamedPipe(
			windows.StringToUTF16Ptr(pipeName), windows.PIPE_ACCESS_DUPLEX,
			windows.PIPE_TYPE_MESSAGE|windows.PIPE_READMODE_MESSAGE|windows.PIPE_WAIT,
			windows.PIPE_UNLIMITED_INSTANCES, 4096, 4096, 0, sa,
		)
		if err != nil {
			LogError("Failed to create pipe: " + err.Error())
			time.Sleep(time.Second)
			continue
		}
		err = windows.ConnectNamedPipe(pipeHandle, nil)
		if err != nil {
			LogError("Failed to connect pipe: " + err.Error())
			windows.CloseHandle(pipeHandle)
			continue
		}
		go handleIPCConnection(pipeHandle)
	}
}
func handleIPCConnection(handle windows.Handle) {
	defer windows.CloseHandle(handle)
	file := os.NewFile(uintptr(handle), "pipe")
	if file == nil {
		return
	}
	defer file.Close()
	_ = file.SetDeadline(time.Now().Add(5 * time.Second))
	var action HelperAction
	if err := json.NewDecoder(file).Decode(&action); err != nil {
		return
	}
	response := ""
	LogInfo("Received action: " + action.Type)
	switch action.Type {
	case "ping":
		response = "pong"
	case "restart":
		LogDebug("Restarting YourPlace Server")
		go restart()
		response = "ok - restarting"
	case "uninstall":
		LogInfo("Uninstalling YourPlace")
		go uninstall(false, false)
		response = "ok - uninstalling"
	case "uninstall -keepUpload":
		LogInfo("Uninstalling YourPlace")
		go uninstall(true, false)
		response = "ok - uninstalling"
	case "uninstall -keepBlockchain":
		LogInfo("Uninstalling YourPlace")
		go uninstall(false, true)
		response = "ok - uninstalling"
	case "uninstall -keepUpload -keepBlockchain":
		LogInfo("Uninstalling YourPlace")
		go uninstall(true, true)
		response = "ok - uninstalling"
	case "update":
		LogInfo("Updating YourPlace Server")
		go update()
		response = "ok - updating"
	case "stop":
		LogInfo("Stopping YourPlace Server")
		go stop()
		response = "ok - stopping"
	case "version":
		response = version
	default:
		LogInfo("Unknown Action")
		response = "unknown"
	}
	json.NewEncoder(file).Encode(response)
}

// Service Actions
func install() bool {
	LogInfo("Installing YourPlace")
	host.InstallScheduledTask(serviceName)
	host.StartScheduledTask(serviceName)
	registerFirewallRule(4002, "YourPlaceIPFS")
	registerFirewallRule(42424, "YourPlace")
	registerUninstaller()
	time.Sleep(5 * time.Second)
	taskInstalled, err := host.IsScheduledTaskInstalled(serviceName)
	if err != nil {
		LogError("Could not check if scheduled task is installed: " + err.Error())
		return false
	}
	if taskInstalled && host.DoesProcExist(serviceName+host.BinaryExtension) {
		LogInfo("YourPlace Helper installed successfully")
		return true
	} else {
		return false
	}
}
func uninstall(keepUploads, keepBlockchain bool) {
	runPowershellUninstaller(keepUploads, keepBlockchain)
	_ = removeUninstaller()
	// uninstallCleanupJob() // try the scheduled task cleanup method
	os.Exit(0)
}
func update() bool {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Get(host.DownloadHost + "/version") // Get latest version
	if err != nil {
		LogWarn("Could not get latest version: " + err.Error())
		return false
	}
	defer resp.Body.Close()
	var versionResp struct {
		Latest string `json:"latest"`
	}
	_err := json.NewDecoder(resp.Body).Decode(&versionResp)
	if _err != nil {
		LogDebug("Could not decode version response: " + _err.Error())
		return false
	}
	currentVersion := host.GetServerVersion() // Get current version
	if currentVersion == "" {
		LogDebug("Could not get current version")
		return false
	}
	if !security.IsNewerVersion(currentVersion, versionResp.Latest) { // Check if newer version is available
		LogDebug("No update available")
		return false
	}
	LogInfo("Downloading update")
	archString := ""
	arch := host.GetCPUVendor()
	if arch == "intel" {
		archString = "amd" + strconv.Itoa(int(host.GetCPUArch()))
	} else if arch == "arm" {
		archString = "arm" + strconv.Itoa(int(host.GetCPUArch()))
	}
	url := host.DownloadHost + "/download?os=windows&arch=" + archString
	type UpdatePayload struct {
		URL string `json:"url"`
		Sig string `json:"sig"`
	}
	var payload UpdatePayload
	err = network.HttpGetJson(url, &payload)
	if err != nil {
		LogWarn("Could not get update payload: " + err.Error())
		return false
	}
	downloadURL := payload.URL
	downloadSig := payload.Sig
	if downloadURL == "" || downloadSig == "" {
		LogError("Could not get download URL or signature")
		return false
	}
	tempDir, err := os.MkdirTemp("", "")
	if err != nil {
		LogError("Could not create temporary directory: " + err.Error())
		return false
	}
	defer os.RemoveAll(tempDir)
	destPath := filepath.Join(tempDir, "YourPlace"+host.BinaryExtension)
	err = host.DownloadFile(downloadURL, destPath)
	if err != nil {
		LogWarn("Could not download update: " + err.Error())
		return false
	}
	defer os.RemoveAll(destPath)
	err = host.DownloadFile(downloadSig, destPath+".sig")
	if err != nil {
		LogWarn("Could not download update signature: " + err.Error())
		return false
	}
	defer os.RemoveAll(destPath + ".sig")
	verified := security.PGPVerifySignature(destPath, destPath+".sig")
	if !verified {
		LogError("Update signature verification failed")
		return false
	}
	host.KillProcess("YourPlace" + host.BinaryExtension)
	if host.DoesProcExist("YourPlace" + host.BinaryExtension) {
		LogError("Could not kill YourPlace process")
		return false
	}
	host.KillProcess("YourPlaceIpfs" + host.BinaryExtension)
	if host.DoesProcExist("YourPlaceIpfs" + host.BinaryExtension) {
		LogError("Could not kill YourPlace IPFS process")
		return false
	}
	host.KillProcess("YourPlaceFfmpeg" + host.BinaryExtension)
	if host.DoesProcExist("YourPlaceFfmpeg" + host.BinaryExtension) {
		LogError("Could not kill YourPlace FFMPEG process")
		return false
	}
	host.DeleteAll(host.GetInstallDir() + "YourPlace" + host.BinaryExtension)
	host.CopyFile(destPath, host.GetInstallDir()+"YourPlace"+host.BinaryExtension)
	err = os.Chmod(destPath, 0600)
	if err != nil {
		LogError("Could not change file permissions: " + err.Error())
		return false
	}
	verified2 := security.PGPVerifySignature(destPath, destPath+".sig")
	if !verified2 {
		LogError("Update signature verification 2 failed")
		host.DeleteAll(host.GetInstallDir() + "YourPlace" + host.BinaryExtension)
		return false
	}
	host.RunShellCommandNoWait(host.GetInstallDir() + "YourPlace" + host.BinaryExtension + " -p=true")

	return true
}
func restart() bool {
	user, _ := GetProcessOwnerAsUser("YourPlace", "YourPlaceIpfs", "YourPlaceHelper", "YourPlaceFfmpeg")
	LogDebug("user: " + user.Name)
	LogDebug("home: " + user.HomeDir)
	execPath := filepath.Join(user.HomeDir, "AppData", "Local", "YourPlace", "YourPlace.exe")
	// Kill running YourPlace processes
	for _, proc := range []string{"YourPlace", "YourPlaceIpfs", "YourPlaceFfmpeg"} {
		procName := proc + host.BinaryExtension
		if host.DoesProcExist(procName) {
			host.KillProcess(procName)
			for i := 0; i < 10; i++ {
				if !host.DoesProcExist(procName) {
					break
				}
				time.Sleep(1 * time.Second)
			}
			if host.DoesProcExist(procName) {
				LogError("Failed to kill process: " + procName)
				return false
			}
		}
	}
	time.Sleep(2 * time.Second)
	// Restart YourPlace executable
	LogDebug("Restart executable: " + execPath)
	for attempt := 1; attempt <= 10; attempt++ {
		err := host.RunAsSpecificUser(user, execPath)
		if err != nil {
			LogDebug("Failed to start YourPlace: " + err.Error())
			time.Sleep(1 * time.Second)
			if host.DoesProcExist("YourPlace" + host.BinaryExtension) {
				LogDebug("Process started but may have crashed")
			}
			continue
		}
		time.Sleep(5 * time.Second)
		if host.DoesProcExist("YourPlace" + host.BinaryExtension) {
			LogInfo("YourPlace restarted successfully")
			return true
		}
	}
	LogDebug("Failed to restart YourPlace after multiple attempts")
	return false
}
func stop() {
	host.StopScheduledTask(serviceName)
	os.Exit(0)
}
func haltRestarter(c *cron.Cron) {
	c.Stop()
}

// Helper Functions
func GetProcessOwnerAsUser(processName string, excludePrefixes ...string) (*user.User, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0) // First, find the process
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)
	var procEntry windows.ProcessEntry32
	procEntry.Size = uint32(unsafe.Sizeof(procEntry))
	if err := windows.Process32First(snapshot, &procEntry); err != nil {
		return nil, err
	}
	counter := 0
	for {
		exeName := windows.UTF16ToString(procEntry.ExeFile[:])
		if strings.HasPrefix(exeName, processName) {
			excluded := false
			for _, excludedPrefix := range excludePrefixes {
				if strings.HasPrefix(exeName, excludedPrefix) {
					excluded = true
					break
				}
			}
			if !excluded {
				return getProcessOwnerByPIDAsUser(procEntry.ProcessID) // Found the process, now get its owner
			}
		}
		if err := windows.Process32Next(snapshot, &procEntry); err != nil {
			break
		}
		counter++
		if counter >= 10000 { //We are iterating through every single running process on the machine so it has to break after an unreasonable amount of processes
			break
		}
	}
	return nil, LogErrorReturn("process not found: " + processName)
}
func getProcessOwnerByPIDAsUser(pid uint32) (*user.User, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION, false, pid)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(handle)
	var token windows.Token
	err = windows.OpenProcessToken(handle, windows.TOKEN_QUERY, &token)
	if err != nil {
		return nil, err
	}
	defer token.Close()
	tokenUser, err := token.GetTokenUser()
	if err != nil {
		return nil, err
	}
	sidString := tokenUser.User.Sid.String()
	account, domain, _, err := tokenUser.User.Sid.LookupAccount("")
	if err != nil {
		return nil, err
	}
	username := account
	if domain != "" && domain != "." {
		username = domain + "\\" + account
	}
	u, err := user.Lookup(account)
	if err != nil {
		if domain != "" && domain != "." { // If that fails, try with domain\account
			u, err = user.Lookup(username)
			if err != nil {
				// If both fail, try looking up by SID
				u, err = user.LookupId(sidString)
				if err != nil {
					return nil, fmt.Errorf("failed to lookup user: %v", err)
				}
			}
		} else {
			return nil, err
		}
	}
	return u, nil
}
func registerUninstaller() {
	uninstallFolder := "C:\\ProgramData\\YourPlace"
	host.CreateFolder(uninstallFolder)
	uninstallBinary := uninstallFolder + host.PathSeparator + "uninstall.ps1"
	host.DeleteIfExists(uninstallBinary)
	host.WriteEmbeddedBinary(uninstallScriptBytes, uninstallBinary)

	keyPath := "SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\" + serviceName
	key, _, err := registry.CreateKey(registry.LOCAL_MACHINE, keyPath, registry.ALL_ACCESS)
	if err != nil {
		log.Println("Error creating uninstaller key: " + err.Error())
		return
	}
	defer key.Close()
	key.SetStringValue("DisplayName", "YourPlace")
	key.SetStringValue("UninstallString", host.GetInstallDir()+"YourPlaceHelper.exe uninstall")
	key.SetStringValue("DisplayIcon", host.GetInstallDir()+"favicon.ico")
	key.SetStringValue("Publisher", "YourPlace Inc.")
	key.SetStringValue("DisplayVersion", "1.0.0")
	key.SetDWordValue("EstimatedSize", 512000) // estimated size in KB
	key.SetDWordValue("NoModify", 1)
	key.SetDWordValue("NoRepair", 1)
}
func removeUninstaller() error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\`+serviceName, registry.ALL_ACCESS)
	if err != nil {
		// Try 32-bit registry view if 64-bit fails
		k, err = registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\`+serviceName, registry.ALL_ACCESS)
		if err != nil {
			return err
		}
	}
	defer k.Close()
	parent, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, registry.ALL_ACCESS)
	if err != nil {
		parent, err = registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`, registry.ALL_ACCESS)
		if err != nil {
			return err
		}
	}
	defer parent.Close()
	return registry.DeleteKey(parent, serviceName)
}
func registerProtocolHandler() {
	execPath := host.GetInstallDir() + "YourPlace.exe"
	cmd := exec.Command("reg", "add", "HKEY_CLASSES_ROOT\\yourplace", "/v", "URL Protocol", "/d", "")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	cmd.Run()
	cmd = exec.Command("reg", "add", "HKEY_CLASSES_ROOT\\yourplace\\shell\\open\\command", "/d", execPath+" %1")
	cmd.Run()
}
func showConfirmationDialog(title, message string) bool {
	messagePtr, _ := syscall.UTF16PtrFromString(message)
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	ret, _, _ := messageBox.Call(0,
		uintptr(unsafe.Pointer(messagePtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(0x00000004|0x00000020)) // MB_YESNO = 0x00000004 | MB_ICONQUESTION = 0x00000020
	return int(ret) == 6 // IDYES
}
func uninstallCleanupJob() {
	folderPath := host.GetInstallDir()
	exePath, err := os.Executable()
	if err != nil {
		core.LogError("Error getting executable path: " + err.Error())
		return
	}
	// Initialize COM
	err = ole.CoInitialize(0)
	if err != nil {
		core.LogError("Error initializing COM: " + err.Error())
		return
	}
	defer ole.CoUninitialize()
	// Create TaskScheduler object
	unknown, err := oleutil.CreateObject("Schedule.Service")
	if err != nil {
		core.LogError("Error creating Schedule.Service: " + err.Error())
		return
	}
	scheduler, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		core.LogError("Error querying interface: " + err.Error())
		return
	}
	defer scheduler.Release()
	// Connect to Task Scheduler
	_, err = oleutil.CallMethod(scheduler, "Connect")
	if err != nil {
		core.LogError("Error connecting to scheduler: " + err.Error())
		return
	}
	// Get root folder
	rootFolder, err := oleutil.CallMethod(scheduler, "GetFolder", "\\")
	if err != nil {
		core.LogError("Error getting root folder: " + err.Error())
		return
	}
	defer rootFolder.ToIDispatch().Release()
	// Create task definition
	taskDef, err := oleutil.CallMethod(scheduler, "NewTask", 0)
	if err != nil {
		core.LogError("Error creating task definition: " + err.Error())
		return
	}
	// Set principal (run as SYSTEM)
	principal := oleutil.MustGetProperty(taskDef.ToIDispatch(), "Principal").ToIDispatch()
	oleutil.MustPutProperty(principal, "UserId", "SYSTEM")
	oleutil.MustPutProperty(principal, "RunLevel", 1) // Highest privileges
	// Create action
	actions := oleutil.MustGetProperty(taskDef.ToIDispatch(), "Actions").ToIDispatch()
	action, err := oleutil.CallMethod(actions, "Create", 0) // TASK_ACTION_EXEC
	if err != nil {
		core.LogError("Error creating action: " + err.Error())
		return
	}
	actionDispatch := action.ToIDispatch()
	oleutil.MustPutProperty(actionDispatch, "Path", "cmd.exe")
	oleutil.MustPutProperty(actionDispatch, "Arguments", fmt.Sprintf("/c rmdir /s /q \"%s\" && del /f /q \"%s\"", folderPath, exePath))
	// Create trigger
	triggers := oleutil.MustGetProperty(taskDef.ToIDispatch(), "Triggers").ToIDispatch()
	trigger, err := oleutil.CallMethod(triggers, "Create", 1) // TASK_TRIGGER_TIME
	if err != nil {
		core.LogError("Error creating trigger: " + err.Error())
		return
	}
	triggerDispatch := trigger.ToIDispatch()
	oleutil.MustPutProperty(triggerDispatch, "StartBoundary", time.Now().Add(time.Second*10).Format("2006-01-02T15:04:05"))
	oleutil.MustPutProperty(triggerDispatch, "Enabled", true)
	// Register the task
	taskName := "CleanupTask"
	_, err = oleutil.CallMethod(rootFolder.ToIDispatch(), "RegisterTaskDefinition", taskName, taskDef.ToIDispatch(), 6, nil, nil, 1, "")
	if err != nil {
		core.LogError("Error registering task: " + err.Error())
		return
	}
	// Run the task
	task, err := oleutil.CallMethod(rootFolder.ToIDispatch(), "GetTask", taskName)
	if err != nil {
		core.LogError("Error getting task: " + err.Error())
		return
	}
	_, err = oleutil.CallMethod(task.ToIDispatch(), "Run", nil)
	if err != nil {
		core.LogError("Error running task: " + err.Error())
		return
	}
	// Delete the task
	_, err = oleutil.CallMethod(rootFolder.ToIDispatch(), "DeleteTask", taskName, 0)
	if err != nil {
		core.LogError("Error deleting task: " + err.Error())
		return
	}
}
func registerFirewallRule(port int, name string) {
	LogDebug("Registering firewall rule: " + name)
	baseParams := []string{ // common rule parameters
		"advfirewall", "firewall", "add", "rule",
		fmt.Sprintf("name=%s", name),
		"action=allow",
		fmt.Sprintf("localport=%d", port),
	}
	configs := []struct {
		protocol  string
		direction string
	}{
		{"TCP", "in"},
		{"TCP", "out"},
		{"UDP", "in"},
		{"UDP", "out"},
	}
	for _, config := range configs {
		params := append([]string{}, baseParams...)
		params = append(params,
			fmt.Sprintf("protocol=%s", config.protocol),
			fmt.Sprintf("dir=%s", config.direction))
		cmd := exec.Command("netsh", params...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		output, err := cmd.CombinedOutput()
		if err != nil {
			LogError("Error registering firewall rule: " + string(output))
		}
	}
	return
}
func removeFirewallRule(port int, name string) {
	LogDebug("Removing firewall rule: " + name)
	baseParams := []string{ // common rule parameters
		"advfirewall", "firewall", "delete", "rule",
		fmt.Sprintf("name=%s", name),
	}
	configs := []struct {
		protocol  string
		direction string
	}{
		{"TCP", "in"},
		{"TCP", "out"},
		{"UDP", "in"},
		{"UDP", "out"},
	}
	for _, config := range configs {
		params := append([]string{}, baseParams...)
		params = append(params,
			fmt.Sprintf("protocol=%s", config.protocol),
			fmt.Sprintf("dir=%s", config.direction))
		cmd := exec.Command("netsh", params...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		output, err := cmd.CombinedOutput()
		if err != nil {
			LogError("Error removing firewall rule: " + string(output))
		}
	}
	return
}
func isVersionUpdated() bool {
	versionFile := host.GetInstallDir() + "helper.version"
	if _, err := os.Stat(versionFile); err != nil {
		return false
	}
	versionBytes, err := os.ReadFile(versionFile)
	if err != nil {
		return false
	}
	versionStr := string(versionBytes)
	return versionStr == version
}
func runPowershellUninstaller(keepUploads, keepBlockchain bool) {
	LogDebug("Running PowerShell uninstaller")
	scriptPath := "C:\\ProgramData\\YourPlace\\uninstall.ps1"
	_, err := os.Stat(scriptPath)
	if err != nil {
		LogError("Uninstall script not found: " + err.Error())
		return
	}
	args := []string{
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-WindowStyle", "Hidden",
		"-File", scriptPath,
	}
	if keepUploads {
		args = append(args, "-keepUpload")
	}
	if keepBlockchain {
		args = append(args, "-keepBlockchain")
	}
	cmd := exec.Command("powershell.exe", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		LogError("Error starting PowerShell process: " + err.Error())
		return
	}
	err = cmd.Process.Release()
	if err != nil {
		LogError("Error releasing PowerShell process: " + err.Error())
		return
	}
	LogInfo("Uninstall script started")
}

// Logging Functions
func LogInit(name string) *os.File {
	user, _ := GetProcessOwnerAsUser("YourPlace", "YourPlaceHelper", "YourPlaceIpfs", "YourPlaceFfmpeg")
	homeDir := user.HomeDir
	logDir := filepath.Join(homeDir, "YourPlace")
	logPath := filepath.Join(logDir, name+".log")
	err := os.MkdirAll(logDir, 0755)
	if err != nil {
		return nil
	}
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("Error opening log file: " + err.Error())
		return nil
	}
	logger = log.New(file, "", log.Ldate|log.Ltime)
	return file
}
func LogInfo(message string) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	fmt.Fprintf(os.Stdout, "%s[INFO]%s %s\n", colorBlue, colorNone, message)
	logger.Printf("[INFO] %s\n", message)
}
func LogDebug(message string) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	fmt.Fprintf(os.Stdout, "%s[DEBUG]%s %s\n", colorPurple, colorNone, message)
	logger.Printf("[DEBUG] %s\n", message)
}
func LogWarn(message string) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	fmt.Fprintf(os.Stdout, "%s[WARN]%s %s\n", colorYellow, colorNone, message)
	logger.Printf("[WARN] %s\n", message)
}
func LogError(message string) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	fmt.Fprintf(os.Stdout, "%s[ERROR]%s %s\n", colorRed, colorNone, message)
	logger.Printf("[ERROR] %s\n", message)
}
func LogFatal(message string) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	fmt.Fprintf(os.Stdout, "%s[FATAL]%s %s\n", colorRed, colorNone, message)
	logger.Printf("[FATAL] %s\n", message)
	os.Exit(1)
}
func LogErrorReturn(message string) error {
	LogError(message)
	return errors.New(message)
}
func LogWarningReturn(message string) error {
	LogWarn(message)
	return errors.New(message)
}
