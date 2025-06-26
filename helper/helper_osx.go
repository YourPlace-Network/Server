//go:build go1.24 && darwin

package main

import (
	"YourPlace/src/core/host"
	"bufio"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type HelperRequest struct {
	Method string `json:"method"`
}
type HelperResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

var (
	logger      *log.Logger
	loggerMutex sync.Mutex
)

const (
	serviceName = "YourPlaceHelper"
	version     = "0.0.1"
	colorRed    = "\033[1;31m"
	colorYellow = "\033[1;33m"
	colorBlue   = "\033[1;34m"
	colorPurple = "\033[1;35m"
	colorNone   = "\033[0m"
)

func main() {
	if !host.CreateMutex(serviceName) { // Singleton pattern
		LogFatal("Another instance of the helper service is already running")
	}
	defer host.ReleaseMutex()
	if !host.IsAdmin() { // Ensure running as administrator
		LogFatal("Helper service must be run as administrator")
	}
	loggerObj := LogInit("yourplacehelper") // Initialize the logger
	if loggerObj == nil {
		fmt.Println("Error initializing logger")
		os.Exit(1)
	}
	LogInfo("Starting YourPlace Helper")
	_ = os.WriteFile(host.GetInstallDir()+"helper.version", []byte(version), 0744) // write the version string to file

	// IPC Server Code
	_ = os.Remove(host.HelperSocketAddr)
	listener, err := net.Listen("unix", host.HelperSocketAddr)
	if err != nil {
		LogFatal("Could not listen on helper socket: " + err.Error())
	}
	defer listener.Close()
	err = os.Chmod(host.HelperSocketAddr, 0666)
	if err != nil {
		LogError("Could not chmod helper socket: " + err.Error())
	}
	for {
		conn, _err := listener.Accept()
		if _err != nil {
			LogDebug("Could not accept connection: " + _err.Error())
			continue
		}
		go handleConnection(conn)
	}
}

/*--- IPC Server Handler --- */
func handleConnection(conn net.Conn) {
	defer func() {
		LogDebug("Closing connection")
		conn.Close()
	}()
	reader := bufio.NewReader(conn)
	requestLine, err := reader.ReadString('\n')
	if err != nil {
		LogDebug("Could not read helper request: " + err.Error())
		return
	}
	var request HelperRequest
	err = json.Unmarshal([]byte(requestLine), &request)
	if err != nil {
		LogDebug("Could not decode helper request: " + err.Error())
		return
	}
	var response HelperResponse
	fullMethod := request.Method
	methodName := strings.Fields(fullMethod)[0] // handles methods with additional params
	switch methodName {
	case "ping":
		log.Println("Received ping request")
		response.Status = "success"
		response.Message = "pong"
	case "restart":
		log.Println("Received restart request")
		_err := restartYourPlaceServer()
		if _err != nil {
			response.Status = "failed"
			response.Message = "could not restart: " + _err.Error()
		} else {
			response.Status = "success"
			response.Message = "restarting"
		}
	case "uninstall":
		LogDebug("Received uninstall request")
		keepUpload := false
		keepBlockchain := false
		if strings.Contains(fullMethod, "keepUpload") {
			keepUpload = true
		}
		if strings.Contains(fullMethod, "keepBlockchain") {
			keepBlockchain = true
		}
		err = uninstallYourPlace(keepUpload, keepBlockchain)
		if err != nil {
			response.Status = "failed"
			response.Message = "could not uninstall yourplace: " + err.Error()
		} else {
			response.Status = "success"
			response.Message = "ok - uninstalling"
		}
	case "version":
		response.Status = "success"
		response.Message = version
	default:
		LogDebug("Received unknown method: " + request.Method)
		response.Status = "failure"
		response.Message = "unknown method"
	}
	responseJSON, err := json.Marshal(response)
	if err != nil {
		log.Println("Could not encode helper response: " + err.Error())
		return
	}
	responseJSON = append(responseJSON, '\n')
	_, err = conn.Write(responseJSON)
	if err != nil {
		log.Println("Could not write helper response: " + err.Error())
		return
	}
}

func restartYourPlaceServer() error {
	_user, err := getProcessUserInfo("YourPlace")
	if err != nil {
		log.Println("Could not find user to launch YourPlace server with: " + err.Error())
		return err
	}
	uid, _ := strconv.Atoi(_user.Uid)
	gid, _ := strconv.Atoi(_user.Gid)
	_ = stopYourPlaceServer()
	for {
		time.Sleep(1 * time.Second)
		_, err = getProcessUserInfo("YourPlace")
		if err != nil {
			break
		}
	}
	time.Sleep(2 * time.Second)
	opener := "/usr/bin/open"
	appPath := "/Applications/YourPlace.app"
	args := []string{opener, appPath}
	env := []string{
		"PATH=/usr/bin:/bin:/usr/sbin:/sbin",
		"USER=" + _user.Username,
		"HOME=" + _user.HomeDir,
		"SHELL=/bin/bash",
		"DISPLAY=:0",
		fmt.Sprintf("LOGNAME=%s", _user.Username),
	}
	procAttr := &syscall.ProcAttr{
		Env: env,           // Pass current working environment
		Dir: _user.HomeDir, // Pass current working directory
		Files: []uintptr{ // Standard file descriptors
			os.Stdin.Fd(),
			os.Stdout.Fd(),
			os.Stderr.Fd(),
		},
		Sys: &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: uint32(uid),
				Gid: uint32(gid),
			},
			Setsid: true, // Create new session
		},
	}
	go func() {
		_, _err := syscall.ForkExec(opener, args, procAttr)
		if _err != nil {
			LogError("Error launching process: " + err.Error())
			return
		}
	}()
	return nil
}
func stopYourPlaceServer() error {
	_user, err := getProcessUserInfo("YourPlace")
	if err != nil {
		log.Println("Could not find user to kill YourPlace server with: " + err.Error())
		return err
	}
	cmd := exec.Command("pkill", "-9", "-u", _user.Username, "YourPlace")
	err = cmd.Run()
	if err != nil {
		log.Println("Could not kill YourPlace server: " + err.Error())
		return err
	}
	return nil
}
func uninstallYourPlace(keepUpload, keepBlockchain bool) error {
	if os.Geteuid() != 0 {
		return LogErrorReturn("Helper is not root, so we can't uninstall YourPlace")
	}
	shell := "/bin/bash"
	script := "/Library/Application Support/YourPlace/uninstall.sh"
	args := []string{shell, script}
	env := os.Environ()
	procAttr := &syscall.ProcAttr{
		Env: env, // Pass current working environment
		Dir: "",  // Pass current working directory
		Files: []uintptr{ // Standard file descriptors
			os.Stdin.Fd(),
			os.Stdout.Fd(),
			os.Stderr.Fd(),
		},
		Sys: &syscall.SysProcAttr{
			Setsid: true, // Create new session
		},
	}
	go func() {
		_, err := syscall.ForkExec(shell, args, procAttr)
		if err != nil {
			LogError("Error launching process: " + err.Error())
			return
		}
	}()
	return nil
}

func getProcessUserInfo(processName string) (*user.User, error) {
	// Get the PID using pgrep
	pidCmd := exec.Command("pgrep", "-x", processName)
	pidOutput, err := pidCmd.Output()
	if err != nil {
		return nil, LogErrorReturn("Could not get PID for process 1: " + err.Error())
	}
	pid := strings.TrimSpace(string(pidOutput))
	if pid == "" {
		return nil, LogErrorReturn("Could not get PID for process 2")
	}
	// Get user info using the PID
	cmd := exec.Command("ps", "-o", "user=", "-p", pid)
	output, err := cmd.Output()
	if err != nil {
		return nil, LogErrorReturn("Could not get user info for process 1: " + err.Error())
	}
	username := strings.TrimSpace(string(output))
	if username == "" {
		return nil, LogErrorReturn("Could not get user info for process 2")
	}
	// Lookup user information
	userInfo, err := user.Lookup(username)
	if err != nil {
		return nil, LogErrorReturn("Could not get user info for process 3: " + err.Error())
	}
	return userInfo, nil
}

// Logging Functions
func LogInit(name string) *os.File {
	user, err := getProcessUserInfo("YourPlace")
	homeDir := user.HomeDir
	logDir := filepath.Join(homeDir, "Library", "Logs", "YourPlace")
	err = os.MkdirAll(logDir, 0755)
	if err != nil {
		fmt.Println("Error creating log directory: " + err.Error())
		return nil
	}
	logFile := filepath.Join(logDir, "yourplacehelper.log")
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
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
