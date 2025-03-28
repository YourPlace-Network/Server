//go:build windows

//go:generate go build -ldflags -H=windowsgui -s -w
//go:generate goversioninfo -icon=src/www/image/favicon.ico

package host

import (
	"YourPlace/src/core"
	"YourPlace/src/core/security"
	"archive/zip"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

//go:embed bin/ipfs/win/ipfs.exe
var ipfsBin []byte

//go:embed bin/ffmpeg/win/ffmpeg.exe
var ffmpegBin []byte

//go:embed bin/base/docker/node-0.8.2.zip
var baseBin []byte

//go:embed bin/helper/win/YourPlaceHelper.exe
var helperBin []byte

//go:embed bin/helper/win/helper.version
var helperVersion []byte

const (
	PathSeparator          = string('\\')
	PathListSeparator      = string(';')
	PowershellRunner       = "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe"
	BinaryExtension        = ".exe"
	pipeName               = `\\.\pipe\yourplacehelper`
	scPath                 = "C:\\Windows\\system32\\sc.exe"
	cmdPath                = "C:\\Windows\\system32\\cmd.exe"
	tasklistPath           = "C:\\Windows\\system32\\tasklist.exe"
	protocolName           = "yourplace"
	DownloadHost           = "https://yourplace.network"
	STARTF_FORCEONFEEDBACK = 0x00000040
)

var (
	user32     = syscall.NewLazyDLL("user32.dll")
	showWindow = user32.NewProc("ShowWindow")
)

type (
	WTS_SESSION_INFO struct {
		SessionID      uint32
		WinStationName *uint16
		State          uint32
	}
	IShellLinkWVtbl struct {
		QueryInterface      uintptr
		AddRef              uintptr
		Release             uintptr
		GetPath             uintptr
		GetIDList           uintptr
		SetIDList           uintptr
		GetDescription      uintptr
		SetDescription      uintptr
		GetWorkingDirectory uintptr
		SetWorkingDirectory uintptr
		GetArguments        uintptr
		SetArguments        uintptr
		GetHotkey           uintptr
		SetHotkey           uintptr
		GetShowCmd          uintptr
		SetShowCmd          uintptr
		GetIconLocation     uintptr
		SetIconLocation     uintptr
		SetRelativePath     uintptr
		Resolve             uintptr
		SetPath             uintptr
	}
	IShellLinkW struct {
		lpVtbl *IShellLinkWVtbl
	}
	IPersistFileVtbl struct {
		QueryInterface uintptr
		AddRef         uintptr
		Release        uintptr
		GetClassID     uintptr
		IsDirty        uintptr
		Load           uintptr
		Save           uintptr
		SaveCompleted  uintptr
		GetCurFile     uintptr
	}
	IPersistFile struct {
		lpVtbl *IPersistFileVtbl
	}
	HelperAction struct {
		Type string `json:"type"`
	}
)

func GetServerID() string {
	cmd := exec.Command("wmic", "cpu", "get", "ProcessorId", "Name", "MaxClockSpeed")
	var out bytes.Buffer
	err := cmd.Run()
	if err != nil {
		core.LogError("Could not generate server ID: " + err.Error())
		return ""
	}
	return security.Hash(out.String())
}
func GetFreeDiskSpace(driveLetter string) uint64 {
	lpFreeBytesAvailable := int64(0)
	lpTotalNumberOfBytes := int64(0)
	lpTotalNumberOfFreeBytes := int64(0)
	kernel32DLL := windows.NewLazySystemDLL("Kernel32.dll")
	getDiskFreeSpaceExW := kernel32DLL.NewProc("GetDiskFreeSpaceExW")
	r, _, errno := syscall.Syscall6(getDiskFreeSpaceExW.Addr(), 4,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(driveLetter))),
		uintptr(unsafe.Pointer(&lpFreeBytesAvailable)),
		uintptr(unsafe.Pointer(&lpTotalNumberOfBytes)),
		uintptr(unsafe.Pointer(&lpTotalNumberOfFreeBytes)), 0, 0)
	if errno != 0 {
		log.Panicf("Could not determine free disk space - syscall errorno: %d\n", errno)
		return 0
	}
	if r != 0 {
		return uint64(lpTotalNumberOfFreeBytes)
	}
	return 0
}
func EscapePath(path string) string {
	escaped := strings.ReplaceAll(path, "\\", "\\\\")
	return escaped
}
func GetSelfFullPath() string {
	sysproc := syscall.MustLoadDLL("kernel32.dll").MustFindProc("GetModuleFileNameW")
	buffer := make([]uint16, syscall.MAX_PATH)
	r, _, err := sysproc.Call(0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	n := uint32(r)
	if n == 0 {
		log.Fatalln(err)
	}
	return string(utf16.Decode(buffer[0:n]))
}
func CreateProcess(command string) {
	// https://stackoverflow.com/questions/39880708/how-to-delete-self-executable-file-in-windows
	// https://stackoverflow.com/questions/1606140/how-can-a-program-delete-its-own-executable
	var sI syscall.StartupInfo
	var pI syscall.ProcessInformation
	argv, err := syscall.UTF16PtrFromString(command)
	if err != nil {
		fmt.Println("Could not create command: " + err.Error())
		return
	}
	err = syscall.CreateProcess(nil, argv, nil, nil,
		false, windows.CREATE_NO_WINDOW,
		nil, nil, &sI, &pI)
	if err != nil {
		fmt.Println("Could not create process: " + err.Error())
		return
	}
}
func Update() bool {
	core.LogInfo("Updating YourPlace")
	_, err := HelperCall("update")
	if err != nil {
		core.LogError("Could not update YourPlace: " + err.Error())
		return false
	}
	return true
}
func Restart() {
	core.LogInfo("Restarting YourPlace Server")
	_, _ = HelperCall("restart")
	Shutdown(0)
}
func CreateShortcut(port int) {
	DeleteIfExists(GetAppDataDir() + "Roaming\\Microsoft\\Windows\\Start Menu\\YourPlace.lnk")
	target := GetInstallDir() + "YourPlace.exe"
	icon := GetInstallDir() + "favicon.ico"
	cmdFormat := "& {\n" +
		"$targetPath = '" + target + "'\n" +
		"$iconPath = '" + icon + "'\n" +
		"$desktopPath = [Environment]::GetFolderPath('%s')\n" +
		"$shortcutPath = Join-Path -Path $desktopPath -ChildPath 'YourPlace.lnk'\n" +
		"$shell = New-Object -ComObject WScript.Shell\n" +
		"$shortcut = $shell.CreateShortcut($shortcutPath)\n" +
		"$shortcut.TargetPath = $targetPath\n" +
		"$shortcut.Arguments = '-s=true'\n" +
		"$shortcut.IconLocation = $iconPath\n" +
		"$shortcut.WindowStyle = 7\n" +
		"$shortcut.Save()}"
	// Create Desktop Shortcut
	cmd := fmt.Sprintf(cmdFormat, "Desktop")
	shell := exec.Command(PowershellRunner, "-NoProfile", "-InputFormat", "None", "-ExecutionPolicy", "Bypass", "-Command", cmd)
	shell.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	_, err := shell.CombinedOutput()
	if err != nil {
		core.LogWarn("Could not create desktop shortcut: " + err.Error())
		return
	}
	// Create Start Menu Shortcut
	cmd2 := fmt.Sprintf(cmdFormat, "StartMenu")
	shell2 := exec.Command(PowershellRunner, "-NoProfile", "-InputFormat", "None", "-ExecutionPolicy", "Bypass", "-Command", cmd2)
	shell2.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	_, err = shell2.CombinedOutput()
	if err != nil {
		core.LogWarn("Could not create desktop shortcut: " + err.Error())
		return
	}
}
func GetPID(processName string) uint32 {
	handle, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0
	}
	processSize := unsafe.Sizeof(windows.ProcessEntry32{})
	proc := windows.ProcessEntry32{Size: uint32(processSize)}
	for {
		err := windows.Process32Next(handle, &proc)
		if err != nil {
			return 0
		}
		if windows.UTF16ToString(proc.ExeFile[:]) == processName {
			return proc.ProcessID
		}
	}
}
func GetAppDataDir() string {
	return "C:\\Users\\" + GetUsername() + "\\AppData\\"
}
func GetUsername() string {
	currentUser, _ := user.Current()
	return currentUser.Username
}
func KillProcess(processName string) bool {
	RunShellCommand("C:\\Windows\\system32\\taskkill.exe /F /T /IM " + processName)
	return !DoesProcExist(processName)
}
func OpenBrowser(url string) {
	if !security.IsValidURL(url) {
		return
	}
	RunShellCommandNoWait("start " + url)
}
func CreateMutex(name string) bool {
	name = "Global\\" + name
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	procCreateMutex := kernel32.NewProc("CreateMutexW")
	ptr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return false
	}
	_, _, err = procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(ptr)))
	switch int(err.(syscall.Errno)) {
	case 0:
		return true
	default:
		return false
	}
}
func ReleaseMutex() {
	// Mutex release is automatically handled by the Windows OS
}
func CreateUninstaller() {
	keyPath := "SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\YourPlace"
	key, _, err := registry.CreateKey(registry.LOCAL_MACHINE, keyPath, registry.ALL_ACCESS)
	if err != nil {
		core.LogError("Could not create uninstaller: " + err.Error())
		return
	}
	defer key.Close()
	err = key.SetStringValue("DisplayName", "YourPlace")
	if err != nil {
		core.LogError("Could not set DisplayName: " + err.Error())
		return
	}
	err = key.SetStringValue("UninstallString", GetInstallDir()+"YourPlaceHelper.exe -number=3")
	if err != nil {
		core.LogError("Could not set UninstallString: " + err.Error())
		return
	}
	err = key.SetDWordValue("NoModify", 1)
	if err != nil {
		core.LogError("Could not set NoModify: " + err.Error())
		return
	}
	_ = key.SetDWordValue("NoRepair", 1)
	if err != nil {
		core.LogError("Could not set NoRepair: " + err.Error())
		return
	}
}
func RunElevatedPowerShell(command string) error {
	// Escape the PowerShell command
	escapedCommand := strings.Replace(command, `"`, `\"`, -1)
	// Construct the PowerShell execution command
	psCommand := fmt.Sprintf(`Start-Process `+PowershellRunner+` -Verb RunAs -ArgumentList "-NoProfile -ExecutionPolicy Bypass -Command %s"`, escapedCommand)
	core.LogWarn("Running elevated PowerShell command: " + psCommand)
	// Create the process
	cmd := exec.Command(PowershellRunner, "-Command", psCommand)
	// Hide the window
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	// Run the command
	return cmd.Run()
}
func RunShellCommand(command string) string {
	return RunShellCommandEnv(command, []string{})
}
func RunShellCommandEnv(command string, env []string) string {
	//core.LogDebug("Running shell command: " + command + "\nWith ENV: " + strings.Join(env, "\n")) // debug
	var stdoutBuffer bytes.Buffer
	_command := security.SanitizeCommandInjection(command)
	cmd := exec.Command("C:\\Windows\\system32\\cmd.exe", "/C", _command)
	cmd.Env = os.Environ()
	for _, variable := range env {
		cmd.Env = append(cmd.Env, variable)
	}
	cmd.Dir, _ = os.Getwd()
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	//cmd.Stdout = os.Stdout // debug
	//cmd.Stderr = os.Stderr // debug
	err := cmd.Run()
	if err != nil {
		return ""
	} else {
		return stdoutBuffer.String()
	}
}
func RunShellCommandNoWait(command string) {
	RunShellCommandNoWaitEnv(command, []string{})
}
func RunShellCommandNoWaitEnv(command string, env []string) {
	//core.LogDebug("Running shell command: " + command + "\nWith ENV: " + strings.Join(env, "\n")) // debug
	_command := security.SanitizeCommandInjection(command)
	cmd := exec.Command("C:\\Windows\\system32\\cmd.exe", "/C", _command)
	cmd.Env = os.Environ()
	for _, variable := range env {
		cmd.Env = append(cmd.Env, variable)
	}
	cmd.Dir, _ = os.Getwd()
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	//cmd.Stdout = os.Stdout // debug
	//cmd.Stderr = os.Stderr // debug
	err := cmd.Start()
	if err != nil {
		core.LogWarn("Could not start command: " + _command + "\n" + err.Error())
	}
}
func RunAsUser(exePath string) error {
	// Run an EXE with drop privileges - this function must be run as admin
	// Initialize COM in MTA mode
	if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err != nil {
		return fmt.Errorf("CoInitializeEx failed: %v", err)
	}
	defer ole.CoUninitialize()
	// Create TaskScheduler object
	unknown, err := oleutil.CreateObject("Schedule.Service")
	if err != nil {
		return fmt.Errorf("failed to create Schedule.Service object: %v", err)
	}
	scheduler, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("failed to get IDispatch interface: %v", err)
	}
	defer scheduler.Release()
	// Connect to Task Scheduler
	if _, err := oleutil.CallMethod(scheduler, "Connect"); err != nil {
		return fmt.Errorf("failed to connect to task scheduler: %v", err)
	}
	// Get root folder
	rootFolder, err := oleutil.CallMethod(scheduler, "GetFolder", "\\")
	if err != nil {
		return fmt.Errorf("failed to get root folder: %v", err)
	}
	defer rootFolder.ToIDispatch().Release()
	// Create task definition
	taskDef, err := oleutil.CallMethod(scheduler, "NewTask", 0)
	if err != nil {
		return fmt.Errorf("failed to create task definition: %v", err)
	}
	defer taskDef.ToIDispatch().Release()
	// Get task settings
	settings := oleutil.MustGetProperty(taskDef.ToIDispatch(), "Settings").ToIDispatch()
	defer settings.Release()
	// Configure settings
	oleutil.MustPutProperty(settings, "Hidden", true)
	oleutil.MustPutProperty(settings, "StartWhenAvailable", true)
	oleutil.MustPutProperty(settings, "DisallowStartIfOnBatteries", false)
	oleutil.MustPutProperty(settings, "StopIfGoingOnBatteries", false)
	oleutil.MustPutProperty(settings, "ExecutionTimeLimit", "PT0S")
	// Create action
	actions := oleutil.MustGetProperty(taskDef.ToIDispatch(), "Actions").ToIDispatch()
	defer actions.Release()
	action, err := oleutil.CallMethod(actions, "Create", 0) // 0 = TASK_ACTION_EXEC
	if err != nil {
		return fmt.Errorf("failed to create action: %v", err)
	}
	actionDisp := action.ToIDispatch()
	defer actionDisp.Release()
	// Set action properties
	oleutil.MustPutProperty(actionDisp, "Path", exePath)
	// Generate unique task name
	taskName := fmt.Sprintf("UserTask_%d", syscall.Getpid())
	// Register the task - running as current user with highest privileges
	_, err = oleutil.CallMethod(rootFolder.ToIDispatch(), "RegisterTaskDefinition", taskName, taskDef.ToIDispatch(), 6, nil, nil, 3, "")
	if err != nil {
		return fmt.Errorf("failed to register task: %v", err)
	}
	// Get the registered task
	task, err := oleutil.CallMethod(rootFolder.ToIDispatch(), "GetTask", taskName)
	if err != nil {
		return fmt.Errorf("failed to get registered task: %v", err)
	}
	defer task.ToIDispatch().Release()
	// Run the task
	_, err = oleutil.CallMethod(task.ToIDispatch(), "Run", nil)
	if err != nil {
		return fmt.Errorf("failed to run task: %v", err)
	}
	// Delete the task
	_, err = oleutil.CallMethod(rootFolder.ToIDispatch(), "DeleteTask", taskName, 0)
	if err != nil {
		return fmt.Errorf("failed to delete task: %v", err)
	}
	return nil
}
func DownloadFile(url string, dest string) error {
	err := os.MkdirAll(dest, os.ModePerm)
	if err != nil {
		return err
	}
	filename := filepath.Base(url)            // Get the filename from the URL
	destPath := filepath.Join(dest, filename) // Create the destination file
	if err != nil {
		return err
	}
	resp, err := http.Get(url) // Send an HTTP GET request to the URL
	if err != nil {
		DeleteIfExists(destPath)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		DeleteIfExists(destPath)
		return core.LogErrorReturn("Unexpected status code when trying to download a file")
	}
	destFile, err := os.Create(destPath)
	defer destFile.Close()
	_, err = io.Copy(destFile, resp.Body) // Copy the response body to the destination file
	if err != nil {
		return err
	}
	return nil
}
func MessageBoxYesNo(title, message string) bool {
	user32 := syscall.MustLoadDLL("user32.dll")
	messageBoxW := user32.MustFindProc("MessageBoxW")
	style := uint32(0x00000004 | 0x00000020 | 0x00001000) // MB_YESNO | MB_ICONQUESTION | MB_TOPMOST
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	messagePtr, _ := syscall.UTF16PtrFromString(message)
	result, _, _ := messageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(messagePtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(style),
	)
	if result == 6 {
		return true
	} else {
		return false
	}
}
func BecomeAdmin() {
	verb := "runas"
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	args := strings.Join(os.Args[1:], " ")
	verbPtr, _ := syscall.UTF16PtrFromString(verb)
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)
	argPtr, _ := syscall.UTF16PtrFromString(args)
	showCmd := int32(1) // SW_NORMAL
	err := windows.ShellExecute(0, verbPtr, exePtr, argPtr, cwdPtr, showCmd)
	if err != nil {
		core.LogFatal("Could not elevate to admin: " + err.Error())
	}
}
func IsAdmin() bool {
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	return err == nil
}
func IsInPath(binary string) bool {
	_, err := exec.LookPath(binary)
	return err == nil
}
func GetKeyboardLayout() string {
	procGetKeyboardLayout := user32.NewProc("GetKeyboardLayout")
	kbLayout, _, _ := procGetKeyboardLayout.Call(0)
	if kbLayout == 0 {
		return "Unknown"
	}
	return strconv.Itoa(int(kbLayout))
}
func DoesProcExist(name string) bool {
	cmd := exec.Command(tasklistPath, "/FI", "IMAGENAME eq "+name)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), name)
}
func OSPreRunHook() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getConsoleWindow := kernel32.NewProc("GetConsoleWindow")
	hwnd, _, _ := getConsoleWindow.Call() // Hide console window
	if hwnd != 0 {
		showWindow.Call(hwnd, syscall.SW_HIDE)
	}
}
func InstallAutorun() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, "Software\\Microsoft\\Windows\\CurrentVersion\\Run", registry.WRITE)
	if err != nil {
		core.LogError("Could not open registry key: " + err.Error())
		return false
	}
	defer key.Close()
	binaryPath := GetCurrentPath() + "YourPlace" + BinaryExtension
	err = key.SetStringValue("YourPlace", binaryPath)
	if err != nil {
		core.LogError("Could not set registry key: " + err.Error())
		return false
	}
	return true
}
func IsOnBattery() bool { //TODO: I cant really test this because I dont have a windows machine with a battery
	cmd := exec.Command("wmic", "path", "Win32_Battery", "get", "DeviceID", "/format:list")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	if !strings.Contains(string(output), "DeviceID") {
		return false
	}
	cmd = exec.Command("wmic", "path", "Win32_Battery", "get", "BatteryStatus")
	output, err = cmd.Output()
	if err != nil {
		core.LogError("Could not get battery status: " + err.Error())
		return false
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return false
	}
	statusLine := strings.TrimSpace(lines[1])
	var status int
	_, err = fmt.Sscanf(statusLine, "%d", &status)
	if err != nil {
		return false
	}
	return !(status == 2 || status == 3 || status == 6 || status == 7)
}
func AddSecret(name string, secret string) {
	cred := wincred.NewGenericCredential(name)
	cred.CredentialBlob = []byte(secret)
	err := cred.Write()
	if err != nil {
		core.LogError("Failed to store secret: " + name)
	}
}
func GetSecret(name string) string {
	cred, err := wincred.GetGenericCredential(name)
	if err != nil {
		core.LogError("Failed to retrieve secret: " + name)
		return ""
	}
	return string(cred.CredentialBlob)
}

// ------ Scheduled Task Functions (Admin) ------ //
func InstallScheduledTask(serviceName string) {
	exePath := GetInstallDir() + "YourPlaceHelper.exe"
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	if err != nil {
		core.LogError("failed to initialize COM: " + err.Error())
		return
	}
	defer ole.CoUninitialize()
	unknown, err := oleutil.CreateObject("Schedule.Service")
	if err != nil {
		core.LogError("failed to create Schedule.Service object: " + err.Error())
		return
	}
	scheduler, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		core.LogError("failed to query scheduler interface: " + err.Error())
		return
	}
	defer scheduler.Release()
	_, err = oleutil.CallMethod(scheduler, "Connect")
	if err != nil {
		core.LogError("failed to connect to scheduler: " + err.Error())
		return
	}
	// Get root folder
	rootFolder, err := oleutil.CallMethod(scheduler, "GetFolder", "\\")
	if err != nil {
		core.LogError("failed to get root folder: " + err.Error())
		return
	}
	folderObj := rootFolder.ToIDispatch()
	defer folderObj.Release()
	// Create task definition
	taskDef, err := oleutil.CallMethod(scheduler, "NewTask", 0)
	if err != nil {
		core.LogError("failed to create new task: " + err.Error())
		return
	}
	taskDefObj := taskDef.ToIDispatch()
	defer taskDefObj.Release()
	// Configure principal (for highest privileges)
	principal, err := oleutil.GetProperty(taskDefObj, "Principal")
	if err != nil {
		core.LogError("failed to get principal: " + err.Error())
		return
	}
	principalObj := principal.ToIDispatch()
	defer principalObj.Release()
	_, err = oleutil.PutProperty(principalObj, "RunLevel", 1) // 1 = Highest privileges
	if err != nil {
		core.LogError("failed to set run level: " + err.Error())
		return
	}
	// Set up the task settings
	settings, err := oleutil.GetProperty(taskDefObj, "Settings")
	if err != nil {
		core.LogError("failed to get settings: " + err.Error())
		return
	}
	settingsObj := settings.ToIDispatch()
	defer settingsObj.Release()
	// Configure settings
	_, err = oleutil.PutProperty(settingsObj, "Enabled", true)
	_, err = oleutil.PutProperty(settingsObj, "Hidden", true) // Run hidden
	_, err = oleutil.PutProperty(settingsObj, "StopIfGoingOnBatteries", false)
	_, err = oleutil.PutProperty(settingsObj, "AllowHardTerminate", false)
	_, err = oleutil.PutProperty(settingsObj, "RunOnlyIfNetworkAvailable", false)
	_, err = oleutil.PutProperty(settingsObj, "DisallowStartIfOnBatteries", false)
	_, err = oleutil.PutProperty(settingsObj, "StopOnIdleEnd", false)
	_, err = oleutil.PutProperty(settingsObj, "ExecutionTimeLimit", "PT0S") // No time limit
	_, err = oleutil.PutProperty(settingsObj, "IdleSettings.StopOnIdleEnd", false)
	_, err = oleutil.PutProperty(settingsObj, "RunOnlyIfIdle", false)
	_, err = oleutil.PutProperty(settingsObj, "IdleSettings.IdleDuration", "PT0S")
	_, err = oleutil.PutProperty(settingsObj, "IdleSettings.WaitTimeout", "PT0S")
	// Create trigger
	triggers, err := oleutil.GetProperty(taskDefObj, "Triggers")
	if err != nil {
		core.LogError("failed to get triggers: " + err.Error())
		return
	}
	triggersObj := triggers.ToIDispatch()
	defer triggersObj.Release()
	trigger, err := oleutil.CallMethod(triggersObj, "Create", 1) // TASK_TRIGGER_TIME
	if err != nil {
		core.LogError("failed to create trigger: " + err.Error())
		return
	}
	triggerObj := trigger.ToIDispatch()
	defer triggerObj.Release()
	// Set up repetition pattern
	repetition, err := oleutil.GetProperty(triggerObj, "Repetition")
	if err != nil {
		core.LogError("failed to get repetition: " + err.Error())
		return
	}
	repetitionObj := repetition.ToIDispatch()
	defer repetitionObj.Release()
	_, err = oleutil.PutProperty(repetitionObj, "Interval", "PT1M") // 1 minute
	if err != nil {
		core.LogError("failed to set interval: " + err.Error())
		return
	}
	// Set start time
	_, err = oleutil.PutProperty(triggerObj, "StartBoundary", time.Now().Format("2006-01-02T15:04:05"))
	if err != nil {
		core.LogError("failed to set start time: " + err.Error())
		return
	}
	// Create action
	actions, err := oleutil.GetProperty(taskDefObj, "Actions")
	if err != nil {
		core.LogError("failed to get actions: " + err.Error())
		return
	}
	actionsObj := actions.ToIDispatch()
	defer actionsObj.Release()
	action, err := oleutil.CallMethod(actionsObj, "Create", 0) // TASK_ACTION_EXEC
	if err != nil {
		core.LogError("failed to create action: " + err.Error())
		return
	}
	actionObj := action.ToIDispatch()
	defer actionObj.Release()
	_, err = oleutil.PutProperty(actionObj, "Path", exePath)
	if err != nil {
		core.LogError("failed to set action path: " + err.Error())
		return
	}
	// Set the action to run hidden
	_, err = oleutil.PutProperty(actionObj, "WorkingDirectory", filepath.Dir(exePath))
	//_, err = oleutil.PutProperty(actionObj, "Arguments", "/background")
	_, err = oleutil.PutProperty(actionObj, "WindowStyle", 7) // 7 = Hidden window
	// Register the task with the highest privileges
	taskResult, err := oleutil.CallMethod(folderObj, "RegisterTaskDefinition",
		serviceName, // Name
		taskDefObj,  // Definition
		6,           // TASK_CREATE_OR_UPDATE
		"SYSTEM",    // User
		nil,         // Password (empty for current user)
		4,           // TASK_LOGON_SERVICE_ACCOUNT
		"",          // No sddl
	)
	if err != nil {
		core.LogError("failed to register task: " + err.Error())
		var oleErr *ole.OleError
		if errors.As(err, &oleErr) {
			core.LogError("OLE Error: " + oleErr.Error())
		}
		return
	}
	defer taskResult.Clear()
	return
}
func IsScheduledTaskInstalled(serviceName string) (bool, error) {
	// Get task info using schtasks command (must be admin)
	cmd := exec.Command("schtasks", "/Query", "/TN", serviceName, "/FO", "LIST")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If the command failed, task likely doesn't exist
		return false, nil
	}
	// Check if output contains task info
	return strings.Contains(string(output), serviceName), nil
}
func StopScheduledTask(serviceName string) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := ole.CoInitialize(0) // Initialize OLE
	if err != nil {
		core.LogError("Failed to initialize OLE: " + err.Error())
		return
	}
	defer ole.CoUninitialize()
	unknown, err := oleutil.CreateObject("Schedule.Service") // Create Schedule.Service object
	if err != nil {
		core.LogError("Failed to create Schedule.Service object: " + err.Error())
		return
	}
	scheduler, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		core.LogError("Failed to query scheduler interface: " + err.Error())
		return
	}
	defer scheduler.Release()
	_, err = oleutil.CallMethod(scheduler, "Connect") // Connect to the scheduler service
	if err != nil {
		core.LogError("Failed to connect to scheduler: " + err.Error())
		return
	}
	rootFolder, err := oleutil.CallMethod(scheduler, "GetFolder", "\\") // Get the root folder
	if err != nil {
		core.LogError("Failed to get root folder: " + err.Error())
		return
	}
	folder := rootFolder.ToIDispatch()
	defer folder.Release()
	task, err := oleutil.CallMethod(folder, "GetTask", serviceName) // Get the task
	if err != nil {
		core.LogError("Failed to get task: " + err.Error())
		return
	}
	taskDispatch := task.ToIDispatch()
	defer taskDispatch.Release()
	_, err = oleutil.CallMethod(taskDispatch, "Stop", 0) // Stop/end the task
	if err != nil {
		core.LogError("Failed to stop task: " + err.Error())
		return
	}
}
func StartScheduledTask(serviceName string) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := ole.CoInitialize(0) // Initialize OLE
	if err != nil {
		core.LogError("Failed to initialize OLE: " + err.Error())
		return
	}
	defer ole.CoUninitialize()
	unknown, err := oleutil.CreateObject("Schedule.Service") // Create Schedule.Service object
	if err != nil {
		core.LogError("Failed to create Schedule.Service object: " + err.Error())
		return
	}
	scheduler, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		core.LogError("Failed to query scheduler interface: " + err.Error())
		return
	}
	defer scheduler.Release()
	_, err = oleutil.CallMethod(scheduler, "Connect") // Connect to the scheduler service
	if err != nil {
		core.LogError("Failed to connect to scheduler: " + err.Error())
		return
	}
	rootFolder, err := oleutil.CallMethod(scheduler, "GetFolder", "\\") // Get the root folder
	if err != nil {
		core.LogError("Failed to get root folder: " + err.Error())
		return
	}
	folder := rootFolder.ToIDispatch()
	defer folder.Release()
	task, err := oleutil.CallMethod(folder, "GetTask", serviceName) // Get the task
	if err != nil {
		core.LogError("Failed to get task: " + err.Error())
		return
	}
	taskDispatch := task.ToIDispatch()
	defer taskDispatch.Release()
	_, err = oleutil.CallMethod(taskDispatch, "Run", 0) // Stop/end the task
	if err != nil {
		core.LogError("Failed to stop task: " + err.Error())
		return
	}
}
func RemoveScheduledTask(serviceName string) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	StopScheduledTask(serviceName) // Stop running task
	err := ole.CoInitialize(0)     // Initialize OLE
	if err != nil {
		core.LogError("Failed to initialize OLE: " + err.Error())
		return
	}
	defer ole.CoUninitialize()
	unknown, err := oleutil.CreateObject("Schedule.Service") // Create Schedule.Service object
	if err != nil {
		core.LogError("Failed to create Schedule.Service object: " + err.Error())
		return
	}
	scheduler, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		core.LogError("Failed to query scheduler interface: " + err.Error())
		return
	}
	defer scheduler.Release()
	_, err = oleutil.CallMethod(scheduler, "Connect") // Connect to the scheduler service
	if err != nil {
		core.LogError("Failed to connect to scheduler: " + err.Error())
		return
	}
	rootFolder, err := oleutil.CallMethod(scheduler, "GetFolder", "\\") // Get the root folder
	if err != nil {
		core.LogError("Failed to get root folder: " + err.Error())
		return
	}
	folder := rootFolder.ToIDispatch()
	defer folder.Release()
	_, err = oleutil.CallMethod(folder, "DeleteTask", serviceName, 0) // Delete the task
	if err != nil {
		core.LogError("Failed to delete task: " + err.Error())
		return
	}
}

// ------ OS Specific Business Logic ------ //
func InstallIPFS() bool {
	KillProcess("YourPlaceIpfs.exe")
	if IsEmbeddedFileEqual(ipfsBin, GetInstallDir()+"YourPlaceIpfs.exe") {
		return true
	}
	if GetCPUArch() == 64 {
		if GetCPUVendor() == "intel" {
			ipfsRepo := GetDataDir() + ".ipfs"
			ipfsPath := "IPFS_PATH=" + ipfsRepo
			WriteEmbeddedBinary(ipfsBin, GetInstallDir()+"YourPlaceIpfs.exe")
			RunShellCommandEnv(GetInstallDir()+"YourPlaceIpfs.exe init", []string{ipfsPath})
			return true
		} else {
			return false
		}
	} else {
		return false
	}
}
func RunIPFS() bool {
	ipfsRepo := GetDataDir() + ".ipfs"
	ipfsPath := "IPFS_PATH=" + EscapePath(ipfsRepo)
	DeleteIfExists(GetDataDir() + ".ipfs" + PathSeparator + "repo.lock")
	go RunShellCommandNoWaitEnv(GetInstallDir()+"YourPlaceIpfs.exe daemon --migrate", []string{ipfsPath})
	return true
}
func InstallFFMPEG() bool {
	KillProcess("YourPlaceFfmpeg.exe")
	if IsEmbeddedFileEqual(ffmpegBin, GetInstallDir()+"YourPlaceFfmpeg.exe") {
		return true
	}
	if GetCPUArch() == 64 {
		WriteEmbeddedBinary(ffmpegBin, GetInstallDir()+"YourPlaceFfmpeg.exe")
		return true
	}
	return false
}
func GetFfmpegBin() string {
	ffmpeg := GetInstallDir() + "YourPlaceFfmpeg.exe"
	return ffmpeg
}
func InstallDocker() bool {
	// https://docs.docker.com/desktop/hardened-desktop/settings-management/configure/
	// https://docs.docker.com/desktop/settings/windows/
	if GetCPUArch() != 64 {
		core.LogError("Docker is only supported on 64-bit systems")
		return false
	}
	installChoice := MessageBoxYesNo("YourPlace", "A full Base node requires Docker Desktop to be installed. Would you like to continue?")
	if !installChoice {
		core.LogInfo("User chose not to install Docker Desktop")
		return false
	}
	destDir := GetInstallDir()
	dockerInstaller := "Docker%20Desktop%20Installer.exe"
	dockerURL := "https://desktop.docker.com/win/main/amd64/149282/" + dockerInstaller
	dockerHash := "https://desktop.docker.com/win/main/amd64/149282/checksums.txt"

	core.LogDebug("Downloading Docker Desktop hash")
	err := DownloadFile(dockerHash, destDir)
	if err != nil {
		core.LogError("Could not download Docker Desktop checksum: " + err.Error())
		return false
	}
	if DoesExist(destDir + dockerInstaller) {
		checksumResult := security.CheckChecksumFile(destDir+"checksums.txt", destDir+dockerInstaller)
		if !checksumResult {
			core.LogDebug("Downloading Docker Desktop")
			err = DownloadFile(dockerURL, destDir)
			if err != nil {
				core.LogError("Could not download Docker Desktop: " + err.Error())
				return false
			}
			checksumResult = security.CheckChecksumFile(destDir+"checksums.txt", destDir+dockerInstaller)
			if !checksumResult {
				core.LogError("Checksum mismatch for Docker Desktop")
				return false
			}
		}
	} else {
		core.LogDebug("Downloading Docker Desktop")
		err = DownloadFile(dockerURL, destDir)
		if err != nil {
			core.LogError("Could not download Docker Desktop: " + err.Error())
			return false
		}
		checksumResult := security.CheckChecksumFile(destDir+"checksums.txt", destDir+dockerInstaller)
		if !checksumResult {
			core.LogError("Checksum mismatch for Docker Desktop")
			return false
		}
	}

	core.LogDebug("Installing Docker Desktop")
	RunShellCommand(destDir + dockerInstaller + " install --quiet --accept-license")
	core.LogDebug("Writing Docker config binaries")
	RunShellCommand("\"" + GetInstallDir() + "YourPlace_Elevator.exe -number=1\"")
	core.LogInfo("Starting Docker Desktop")
	go RunShellCommand("C:\\\"Program Files\"\\Docker\\Docker\\\"Docker Desktop.exe\" --minimized --skip-survey")
	return true
}
func IsDockerSocketExist() bool {
	conn, err := net.DialTimeout("tcp", "localhost:2375", 2*time.Second)
	if err != nil {
		return false
	} else {
		defer conn.Close()
		return true
	}
}
func InstallGethNode() bool {
	if GetCPUArch() != 64 {
		core.LogError("Geth is only supported on 64-bit systems")
		return false
	}
	core.LogDebug("Installing Geth node")
	return false
	//WriteEmbeddedBinary(gethBin, GetInstallDir()+"geth-library.zip")
	gethInstallerPath := GetInstallDir() + "geth-library.zip"
	gethPath := GetInstallDir() + "geth.exe"
	DeleteIfExists(gethPath)
	zipFile, err := zip.OpenReader(gethInstallerPath)
	if err != nil {
		core.LogError("Error opening zip file: " + err.Error())
		return false
	}
	defer zipFile.Close()
	zipFileDir := filepath.Dir(gethInstallerPath)
	for _, unzippedFile := range zipFile.File { // Iterate over each file in the zip archive
		if strings.HasSuffix(unzippedFile.Name, "geth.exe") {
			zipFileReader, err2 := unzippedFile.Open()
			if err2 != nil {
				fmt.Println("Error opening file in zip archive: " + err2.Error())
				continue
			}
			defer zipFileReader.Close()
			destinationFilePath := filepath.Join(zipFileDir, filepath.Base(unzippedFile.Name))
			destinationFile, err3 := os.OpenFile(destinationFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, unzippedFile.Mode())
			if err3 != nil {
				fmt.Println("Error creating destination file: " + err3.Error())
				continue
			}
			defer destinationFile.Close()
			_, err4 := io.Copy(destinationFile, zipFileReader)
			if err4 != nil {
				fmt.Println("Error copying file contents: " + err4.Error())
				continue
			}
			InstallLighthouse() // Install ETH consensus client
			// todo
			return true
		}
	}
	core.LogError("File 'geth.exe' not found in the zip archive")
	return false
}
func InstallLighthouse() bool {
	version := "5.1.3"
	binaryURL := "https://github.com/sigp/lighthouse/releases/download/v" + version + "/lighthouse-v" + version + "-x86_64-windows-portable.tar.gz"
	pgpSigURL := "https://github.com/sigp/lighthouse/releases/download/v" + version + "/lighthouse-v" + version + "-x86_64-windows.tar.gz.asc"
	err := DownloadFile(binaryURL, GetInstallDir()+"lighthouse.tar.gz")
	if err != nil {
		core.LogError("Could not download Lighthouse binary: " + err.Error())
		return false
	}
	err = DownloadFile(pgpSigURL, GetInstallDir()+"lighthouse.tar.gz.asc")
	if err != nil {
		core.LogError("Could not download Lighthouse PGP signature: " + err.Error())
		return false
	}
	if !security.PGPVerifySignature(GetInstallDir()+"lighthouse.tar.gz.asc", GetInstallDir()+"lighthouse.tar.gz") {
		core.LogError("Could not verify Lighthouse PGP signature")
		return false
	}
	UntarFile(GetInstallDir()+"lighthouse.tar.gz", GetInstallDir())

	return false
}
func RunGethNode(port int, dataDir string) bool {
	gethPath := GetInstallDir() + "geth.exe"
	if !DoesExist(dataDir) {
		core.LogError("Geth data directory does not exist: " + dataDir)
		return false
	}
	params := "--datadir " + dataDir + " --syncmode full --http --http.addr 127.0.0.1 --http.port " + strconv.Itoa(port) + " --http.api eth,net,web3"
	go RunShellCommandNoWait(gethPath + " " + params)
	return true
}
func InstallRunBaseNode() bool {
	core.LogDebug("Installing base node")
	InstallGethNode()
	//RunGethNode(8545, GetInstallDir()+"geth") todo
	if !IsDockerSocketExist() {
		core.LogDebug("Docker socket not found, installing Docker Desktop")
		dockerInstalled := InstallDocker()
		if !dockerInstalled {
			core.LogError("Could not install Docker")
			return false
		}
	}
	WriteEmbeddedBinary(baseBin, GetInstallDir()+"base-node.zip")
	err := UnzipFile(GetInstallDir()+"base-node.zip", GetInstallDir())
	if err != nil {
		core.LogError("Could not unzip base node: " + err.Error())
		return false
	}

	return false
}
func InstallWSLUbuntu() bool {
	if !IsAdmin() {
		core.LogWarn("WSL installation requires admin privileges")
		return false
	}
	if !IsInPath("wsl.exe") {
		// Enable WSL
		RunShellCommand("powershell -Command dism.exe /online /enable-feature /featurename:Microsoft-Windows-Subsystem-Linux /all /norestart")
		// Enable Virtual Machine Platform
		RunShellCommand("powershell -Command dism.exe /online /enable-feature /featurename:VirtualMachinePlatform /all /norestart")
	}
	RunShellCommand("wsl --set-default-version 2")
	// Install Ubuntu
	RunShellCommand("wsl --install -d Ubuntu")
	return true
}
func RunWSLUbuntuCmd(command string) string {
	sanitizedCmd := security.SanitizeCommandInjection(command)
	sanitizedCmd = security.SanitizePathTraversal(sanitizedCmd)
	cmd := exec.Command("wsl", "-d", "Ubuntu", "--", sanitizedCmd)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	output, err := cmd.CombinedOutput()
	if err != nil {
		core.LogError("WSL command failed: " + err.Error())
		return ""
	}
	return strings.TrimSpace(string(output))
}

// --- Helper Functions --- //
func InstallHelper() bool {
	name := "YourPlaceHelper"
	binary := name + BinaryExtension
	helperPath := GetInstallDir() + binary
	if helperBin == nil {
		core.LogError("Helper binary not embedded")
		Shutdown(1)
		return false
	}
	// Ensure install directory exists
	installDir := filepath.Dir(helperPath)
	err := os.MkdirAll(installDir, 0755)
	if err != nil {
		core.LogError("Failed to create install directory: " + err.Error())
		return false
	}
	// Check if helper needs update by comparing binary contents
	installedHelperVersionBytes, _ := os.ReadFile(GetInstallDir() + "helper.version")
	needsUpdate := false
	if !bytes.Equal(installedHelperVersionBytes, helperVersion) {
		needsUpdate = true
	}
	//needsUpdate := !IsEmbeddedFileEqual(helperBin, helperPath)
	core.LogDebug("Helper needs update: " + strconv.FormatBool(needsUpdate))
	// If helper is running and needs update, stop it first
	if needsUpdate && DoesProcExist(binary) {
		_, err = HelperCall("stop")
		if err == nil {
			// Wait for helper to stop
			for i := 0; i < 30; i++ {
				if !DoesProcExist(binary) {
					core.LogDebug("Helper stopped successfully")
					break
				}
				time.Sleep(2 * time.Second)
			}
		}
		// Check that the helper stopped as expected
		if DoesProcExist(binary) {
			core.LogError("Could not stop helper process")
			return false
		}
	}
	// Update helper binary if needed
	if needsUpdate {
		// Write to temporary file first
		tempPath := helperPath + ".tmp"
		err = os.WriteFile(tempPath, helperBin, 0744)
		if err != nil {
			core.LogError("Could not write temporary helper binary: " + err.Error())
			return false
		}
		// Atomically rename temp file to final location
		err = os.Rename(tempPath, helperPath)
		if err != nil {
			os.Remove(tempPath)
			core.LogError("Could not replace helper binary: " + err.Error())
			return false
		}
		// Run elevated installer for new binary
		core.LogDebug("Starting helper installation")
		verbPtr, _ := windows.UTF16PtrFromString("runas")
		exePtr, _ := windows.UTF16PtrFromString(helperPath)
		paramPtr, _ := windows.UTF16PtrFromString("install")
		cwdPtr, _ := syscall.UTF16PtrFromString(GetInstallDir())
		err = windows.ShellExecute(0, verbPtr, exePtr, paramPtr, cwdPtr, windows.SW_HIDE)
		if err != nil {
			core.LogError("Could not start helper installation: " + err.Error())
			return false
		}
		time.Sleep(5) // Wait for helper to start and UAC prompt
	}
	// Verify helper is running and responding
	core.LogDebug("Verifying installation - Waiting for helper to start")
	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		// Check process
		if !DoesProcExist(binary) {
			core.LogDebug("Process not yet running")
			time.Sleep(2 * time.Second)
			continue
		}
		// Verify IPC communication
		core.LogDebug("Checking IPC communication")
		response, err := HelperCall("ping")
		if err != nil {
			core.LogDebug(fmt.Sprintf("Ping failed: %v", err))
			time.Sleep(time.Second)
			continue
		}
		if response == "pong" {
			core.LogDebug("Helper installation verified successfully")
			return true
		} else {
			core.LogDebug("Unexpected response: " + response)
		}
		time.Sleep(time.Second)
	}
	// Log diagnostic information if installation failed
	core.LogDebug("Helper installation verification failed")
	return false
}

// windows.go - Client side
func HelperCall(action string) (string, error) {
	core.LogDebug("Calling helper with action: " + action)
	// Timeout for entire operation
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Channel for results
	type result struct {
		response string
		err      error
	}
	resultsCh := make(chan result, 1)

	go func() {
		var pipe *os.File
		var err error
		// Connection retry loop
		for i := 0; i < 120; i++ {
			pipe, err = os.OpenFile(pipeName, os.O_RDWR, 0)
			if err == nil {
				break
			}
			select {
			case <-ctx.Done():
				resultsCh <- result{"", core.LogErrorReturn("Context cancelled while attempting to open pipe")}
				return
			case <-time.After(2 * time.Second):
			}
		}
		defer pipe.Close()
		// Set deadline for both read and write operations
		pipe.SetDeadline(time.Now().Add(5 * time.Second))
		// Write request
		if err = json.NewEncoder(pipe).Encode(HelperAction{Type: action}); err != nil {
			resultsCh <- result{"", core.LogErrorReturn("Could not write to named pipe: " + err.Error())}
			return
		}
		// Read response
		var response string
		if err = json.NewDecoder(pipe).Decode(&response); err != nil {
			resultsCh <- result{"", core.LogErrorReturn("Failed to decode response: " + err.Error())}
			return
		}
		resultsCh <- result{response, nil}
	}()
	// Wait for either completion or timeout
	select {
	case <-ctx.Done():
		return "", core.LogErrorReturn("Helper call timed out")
	case res := <-resultsCh:
		return res.response, res.err
	}
}
