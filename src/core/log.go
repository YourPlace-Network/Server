package core

import ( // don't add any exported YourPlace functions - it will create a dependency loop
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const (
	colorRed    = "\033[1;31m"
	colorGreen  = "\033[1;32m"
	colorYellow = "\033[1;33m"
	colorBlue   = "\033[1;34m"
	colorPurple = "\033[1;35m"
	colorCyan   = "\033[1;36m"
	colorOrange = "\033[1;202m"
	colorNone   = "\033[0m"
	maxFileSize = 100 * 1024 * 1024 // 100MB in bytes
	maxBackups  = 5
)

var (
	logger      *log.Logger
	loggerMutex sync.Mutex
	currentFile *os.File
	helperFile  *os.File
)

// getLogDirectory returns the appropriate log directory for the current OS
func getLogDirectory() string {
	if runtime.GOOS == "darwin" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Logs", "YourPlace")
	} else if runtime.GOOS == "windows" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "YourPlace")
	}
	// Default fallback for other OS
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "YourPlace")
}

func LogInit(name string) *os.File {
	var logDir string
	if name == "yourplacesnapshot" {
		homedir, _ := os.UserHomeDir()
		logDir = filepath.Join(homedir, "YourPlaceSnapshot")
	} else {
		logDir = getLogDirectory()
	}
	err := os.MkdirAll(logDir, 0755)
	if err != nil {
		log.Fatal("Failed to create log directory: " + err.Error())
	}
	logPath := filepath.Join(logDir, name+".log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal("Error opening log file: " + err.Error())
	}
	currentFile = file
	logger = log.New(file, "", log.Ldate|log.Ltime)
	return file
}
func LogInfo(message string) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	_, _ = fmt.Fprintf(os.Stdout, "%s[INFO]%s %s\n", colorBlue, colorNone, message)
	logger.Printf("[INFO] %s\n", message)
}
func LogDebug(message string) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	if os.Getenv("YourPlaceDebug") == "true" || os.Getenv("YourPlaceSnapshotDebug") == "true" {
		_, _ = fmt.Fprintf(os.Stdout, "%s[DEBUG]%s %s\n", colorPurple, colorNone, message)
		logger.Printf("[DEBUG] %s\n", message)
	}
}
func LogWarn(message string) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	_, _ = fmt.Fprintf(os.Stdout, "%s[WARN]%s %s\n", colorYellow, colorNone, message)
	logger.Printf("[WARN] %s\n", message)
}
func LogError(message string) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	_, _ = fmt.Fprintf(os.Stdout, "%s[ERROR]%s %s\n", colorRed, colorNone, message)
	logger.Printf("[ERROR] %s\n", message)
}
func LogFatal(message string) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	_, _ = fmt.Fprintf(os.Stdout, "%s[FATAL]%s %s\n", colorRed, colorNone, message)
	logger.Printf("[FATAL] %s\n", message)
	os.Exit(1)
}
func LogLevel(level, message string) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	_, _ = fmt.Fprintf(os.Stdout, "%s[%s]%s %s\n", colorCyan, level, colorNone, message)
	logger.Printf("[%s] %s\n", level, message)
}
func LogErrorReturn(message string) error {
	LogError(message)
	return errors.New(message)
}
func LogWarningReturn(message string) error {
	LogWarn(message)
	return errors.New(message)
}
func LogDebugReturn(message string) error {
	LogDebug(message)
	return errors.New(message)
}
func LogRead(lines int, newlineFlag int) (string, string) { // Return the latest X lines from the log file - newLineFlag: 1=<br>, 2=\n, 3=\r\n
	newline := ""
	switch newlineFlag {
	case 1:
		newline = "<br>"
	case 2:
		newline = "\n"
	case 3:
		newline = "\r\n"
	}
	file, err := os.Open(currentFile.Name())
	if err != nil {
		LogError("Failed to open log file: " + err.Error())
		return "", ""
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	buffer := make([]string, lines)
	position := 0
	lineCount := 0
	for scanner.Scan() {
		buffer[position] = scanner.Text()
		position = (position + 1) % lines
		lineCount++
	}
	err = scanner.Err()
	if err != nil {
		LogError("Failed to read log file: " + err.Error())
		return "", ""
	}
	var result strings.Builder
	if lineCount < lines {
		for i := lineCount - 1; i >= 0; i-- {
			result.WriteString(buffer[i])
			result.WriteString(newline)
		}
	} else {
		// We have at least 'lines' lines, so we need to arrange them in order
		for i := lines - 1; i >= 0; i-- {
			idx := (position + i) % lines
			result.WriteString(buffer[idx])
			result.WriteString(newline)
		}
	}
	return result.String(), currentFile.Name()
}
func LogReadHelper(lines int, newlineFlag int) (string, string) { // Return the latest X lines from the helper log file - newLineFlag: 1=<br>, 2=\n, 3=\r\n
	logPath := filepath.Join(getLogDirectory(), "yourplacehelper.log")
	newline := ""
	switch newlineFlag {
	case 1:
		newline = "<br>"
	case 2:
		newline = "\n"
	case 3:
		newline = "\r\n"
	}
	file, err := os.Open(logPath)
	if err != nil {
		LogError("Failed to open log file: " + err.Error())
		return "", ""
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	buffer := make([]string, lines)
	position := 0
	lineCount := 0
	for scanner.Scan() {
		buffer[position] = scanner.Text()
		position = (position + 1) % lines
		lineCount++
	}
	err = scanner.Err()
	if err != nil {
		LogError("Failed to read log file: " + err.Error())
		return "", ""
	}
	var result strings.Builder
	if lineCount < lines {
		for i := lineCount - 1; i >= 0; i-- {
			result.WriteString(buffer[i])
			result.WriteString(newline)
		}
	} else {
		// We have at least 'lines' lines, so we need to arrange them in order
		for i := lines - 1; i >= 0; i-- {
			idx := (position + i) % lines
			result.WriteString(buffer[idx])
			result.WriteString(newline)
		}
	}
	return result.String(), logPath
}
