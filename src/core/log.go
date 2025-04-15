package core

import ( // don't add any exported YourPlace functions - it will create a dependency loop
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
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
)

func rotateLogFile(baseFilePath string) (*os.File, error) {
	if currentFile != nil { // Close existing file if open
		_ = currentFile.Close()
	}
	// Check if the current log file exists and its size
	info, err := os.Stat(baseFilePath)
	if err == nil && info.Size() >= maxFileSize {
		// Rotate existing backup files
		for i := maxBackups - 1; i >= 0; i-- {
			oldPath := fmt.Sprintf("%s.%d", baseFilePath, i)
			newPath := fmt.Sprintf("%s.%d", baseFilePath, i+1)
			// Remove the oldest backup if it exists
			if i == maxBackups-1 {
				_ = os.Remove(oldPath)
			}
			// Rename existing backups
			_, err = os.Stat(oldPath)
			if err == nil {
				_ = os.Rename(oldPath, newPath)
			}
		}
		// Rename current log file to .1
		_ = os.Rename(baseFilePath, baseFilePath+".1")
	}
	// Open/Create new log file
	file, err := os.OpenFile(baseFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	return file, nil
}
func checkRotate() error {
	if currentFile == nil {
		return errors.New("log file not initialized")
	}
	info, err := os.Stat(currentFile.Name())
	if err != nil {
		return fmt.Errorf("failed to get log file info: %w", err)
	}
	if info.Size() >= maxFileSize {
		file, _err := rotateLogFile(currentFile.Name())
		if _err != nil {
			return fmt.Errorf("failed to rotate log file: %w", _err)
		}
		currentFile = file
		logger = log.New(file, "", log.Ldate|log.Ltime)
	}
	return nil
}

func LogInit(name string) *os.File {
	user, _ := os.UserHomeDir()
	logDir := filepath.Join(user, "YourPlace")
	_ = os.MkdirAll(logDir, 0755)
	logPath := filepath.Join(logDir, name+".log")
	file, err := rotateLogFile(logPath)
	if err != nil {
		LogError("Failed to rotate log file: " + err.Error())
		return nil
	}
	currentFile = file
	logger = log.New(file, "", log.Ldate|log.Ltime)
	return file
}
func LogInfo(message string) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	_ = checkRotate()
	_, _ = fmt.Fprintf(os.Stdout, "%s[INFO]%s %s\n", colorBlue, colorNone, message)
	logger.Printf("[INFO] %s\n", message)
}
func LogDebug(message string) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	_ = checkRotate()
	if os.Getenv("YourPlaceDebug") == "true" {
		_, _ = fmt.Fprintf(os.Stdout, "%s[DEBUG]%s %s\n", colorPurple, colorNone, message)
		logger.Printf("[DEBUG] %s\n", message)
	}
}
func LogWarn(message string) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	_ = checkRotate()
	_, _ = fmt.Fprintf(os.Stdout, "%s[WARN]%s %s\n", colorYellow, colorNone, message)
	logger.Printf("[WARN] %s\n", message)
}
func LogError(message string) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	_ = checkRotate()
	_, _ = fmt.Fprintf(os.Stdout, "%s[ERROR]%s %s\n", colorRed, colorNone, message)
	logger.Printf("[ERROR] %s\n", message)
}
func LogFatal(message string) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	_ = checkRotate()
	_, _ = fmt.Fprintf(os.Stdout, "%s[FATAL]%s %s\n", colorRed, colorNone, message)
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
func LogRead(lines int, newlineFlag int) (string, string) { // Return the latest X lines from the log file
	// newLineFlag: 1=<br>, 2=\n, 3=\r\n
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
