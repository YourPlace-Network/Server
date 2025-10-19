package host

import (
	_core "YourPlace/src/core"
	"YourPlace/src/core/security"
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"embed"
	"errors"
	"io"
	"io/fs"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/oschwald/geoip2-golang"
)

//go:embed bin/GeoLite2-Country.mmdb
var geoliteDB []byte

func IsDebugMode() bool {
	if GetEnvVar("YourPlaceDebug") == "true" || DoesExist(GetDataDir()+"debug") {
		return true
	}
	return false
}
func SetDebugMode(status bool) {
	if status {
		if !IsDebugMode() {
			CreateFile("", GetDataDir()+"debug")
			Restart()
		}
	} else {
		if IsDebugMode() {
			DeleteIfExists(GetDataDir() + "debug")
			Restart()
		}
	}
}
func GetServerVersion() string {
	content, err := os.ReadFile(GetInstallDir() + "yourplace.version")
	if err != nil {
		return ""
	}
	if security.IsValidYourPlaceVersion(string(content)) {
		return string(content)
	}
	return ""
}
func GetInstallDir() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		_core.LogFatal("Could not get OS data directory: " + err.Error())
	}
	path := cacheDir + PathSeparator + "YourPlace" + PathSeparator
	return path
}
func GetDataDir() string {
	if GetEnvVar("YourPlaceGateway") == "true" {
		return "/opt/YourPlace" + string(PathSeparator)
	}
	path := GetHomeDir() + string(PathSeparator) + "YourPlace" + string(PathSeparator)
	return path
}
func GetHomeDir() string {
	userName, err := os.UserHomeDir()
	if err != nil {
		_core.LogError("Could not get home directory: " + err.Error())
	}
	return userName
}
func GetCPUArch() uint32 {
	if strings.Contains(runtime.GOARCH, "64") {
		return 64
	} else {
		return 32
	}
}
func GetCPUVendor() string {
	arch := runtime.GOARCH
	if arch == "amd64" || arch == "386" {
		return "intel"
	} else if strings.Contains(arch, "arm") {
		return "arm"
	}
	return "unknown"
}
func GetUsername() string {
	currentUser, _ := user.Current()
	return currentUser.Username
}
func GetFileExtenstion(directory string, fileName string) (string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		_core.LogError("Could not read directory: " + directory + ": " + err.Error())
		return "", _core.LogErrorReturn("Could not read directory: " + directory + ": " + err.Error())
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		nameFromDisk := entry.Name()
		extension := filepath.Ext(nameFromDisk)
		nameWithoutExtension := nameFromDisk[:len(nameFromDisk)-len(extension)]
		if nameWithoutExtension == fileName {
			return extension, nil
		}
	}
	return "", _core.LogErrorReturn("could not get file extension")
}
func DoesExist(path string) bool {
	fileInfo, err := os.Stat(path)
	if err == nil { // the file can be opened
		if fileInfo.IsDir() {
			return true
		}
		if fileInfo.Size() >= 1 { // the file is over 1 byte
			return true
		}
	}
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	return false
}
func DeleteIfExists(path string) {
	if DoesExist(path) {
		DeleteAll(path)
	}
}
func CreateFolder(path string) {
	err := os.MkdirAll(path, 0755)
	if err != nil {
		_core.LogError("Could not create folder: " + path + " - " + err.Error())
	}
}
func GetTempDir() string {
	return os.TempDir() + PathSeparator
}
func GetSelfBinPath() string {
	execPath, err := os.Executable()
	if err != nil {
		_core.LogError("Could not get self executable path: " + err.Error())
		return ""
	}
	return execPath
}
func DeleteAll(path string) {
	err := os.RemoveAll(path)
	if err != nil {
		_core.LogDebug("Could not delete: " + err.Error())
	}
}
func GetFileSize(path string) int64 {
	fileInfo, err := os.Stat(path)
	if err != nil {
		_core.LogError("Could not get file size: " + err.Error())
		return 0
	}
	return fileInfo.Size() // returns file size in number of bytes
}
func GetCurrentPath() string {
	path, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return path
}
func GetSelfPID() uint32 {
	return uint32(os.Getppid())
}
func GetSelfName() string {
	executable, err := os.Executable()
	if err != nil {
		panic(err)
	}
	file, err := os.Stat(executable)
	if err != nil {
		panic(err)
	}
	return file.Name()
}
func GetOS() string {
	return runtime.GOOS
}
func GetEnvVar(key string) string {
	return os.Getenv(key)
}
func GetGeoIPCountryCode(ipAddress string) string {
	db, err := geoip2.FromBytes(geoliteDB)
	if err != nil {
		return ""
	}
	defer db.Close()
	ip := net.ParseIP(ipAddress)
	if ip == nil {
		return ""
	}
	record, err := db.Country(ip)
	if err != nil {
		return ""
	}
	return record.Country.IsoCode
}
func SetEnvVar(key string, value string) {
	err := os.Setenv(key, value)
	if err != nil {
		_core.LogError("Could not set environment variable: " + err.Error())
	}
}
func DeleteEnvVar(key string) {
	err := os.Unsetenv(key)
	if err != nil {
		_core.LogWarn("Could not delete environment variable: " + err.Error())
	}
}
func IsPIDRunning(pid uint32) bool {
	_, err := os.FindProcess(int(pid))
	if err != nil {
		return false
	}
	return true
}
func IsDirWriteable(path string) bool {
	f, err := os.CreateTemp(path, "test")
	if err != nil {
		return false
	} else {
		err2 := f.Close()
		if err2 != nil {
			return false
		}
		DeleteIfExists(f.Name())
		return true
	}
}
func CopyFile(source, destination string) bool {
	sourceFileStat, err := os.Stat(source)
	if err != nil {
		return false
	}
	if !sourceFileStat.Mode().IsRegular() {
		return false
	}
	src, err := os.Open(source)
	if err != nil {
		return false
	}
	defer src.Close()
	dest, err := os.Create(destination)
	if err != nil {
		return false
	}
	defer dest.Close()
	_, err = io.Copy(dest, src)
	if err != nil {
		return false
	}
	return true
}
func CreateFile(payload string, path string) {
	file, err := os.Create(path)
	if err != nil {
		panic("Could not create create file: " + err.Error())
		return
	} else {
		_, err := file.WriteString(payload)
		if err != nil {
			panic("Could not write to file: " + err.Error())
			return
		}
	}
	err = file.Close()
	if err != nil {
		panic("Could not close file handle: " + err.Error())
		return
	}
}
func IsEmbeddedFileEqual(embeddedFile []byte, diskFilePath string) bool {
	diskData, err := os.ReadFile(diskFilePath)
	if err != nil {
		return false
	}
	return bytes.Equal(embeddedFile, diskData)
}
func UnzipFile(path string, destination string) error {
	zipFile, err := zip.OpenReader(path)
	if err != nil {
		return _core.LogErrorReturn("Could not get zip file: " + err.Error())
	}
	defer zipFile.Close()
	for _, file := range zipFile.File {
		rc, err := file.Open()
		if err != nil {
			return _core.LogErrorReturn("Could not open zip file: " + err.Error())
		}
		defer rc.Close()
		path := filepath.Join(destination, file.Name)
		if file.FileInfo().IsDir() {
			os.MkdirAll(path, file.Mode())
		} else {
			os.MkdirAll(filepath.Dir(path), file.Mode())
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
			if err != nil {
				return _core.LogErrorReturn("Could not open file for unzipping: " + err.Error())
			}
			defer f.Close()
			_, err = io.Copy(f, rc)
			if err != nil {
				return _core.LogErrorReturn("Could not copy contents of the zip file to the new file on disk: " + err.Error())
			}
		}
	}
	return nil
}
func UntarFile(path string, destination string) {
	file, err := os.Open(path)
	if err != nil {
		_core.LogError("could not open tar file: " + err.Error())
		return
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		_core.LogError("could not open gzip file: " + err.Error())
		return
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			_core.LogError("error untarring file: " + err.Error())
			return
		}
		filePath := filepath.Join(destination, header.Name)
		if header.FileInfo().IsDir() {
			err := os.MkdirAll(filePath, os.FileMode(header.Mode))
			if err != nil {
				_core.LogError("could not create directory: " + err.Error())
				return
			}
		} else {
			// Ensure parent directory exists
			err := os.MkdirAll(filepath.Dir(filePath), 0755)
			if err != nil {
				_core.LogError("could not create parent directory: " + err.Error())
				return
			}
			file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				_core.LogError("could not create new file to untar to: " + err.Error())
				return
			}
			if _, err := io.Copy(file, tarReader); err != nil {
				_core.LogError("could not copy contents to the destination untar file: " + err.Error())
				file.Close()
				return
			}
			file.Close()
		}
	}
}
func GetDomainName() string {
	host, err := os.Hostname()
	if err != nil {
		_core.LogWarn("Could not get hostname: " + err.Error())
		return ""
	}
	addrs, err := net.LookupIP(host)
	if err != nil {
		_core.LogWarn("Could not get IP address: " + err.Error())
		return ""
	}
	for _, addr := range addrs {
		ptrs, err := net.LookupAddr(addr.String())
		if err == nil {
			// look for the 1st PTR record that contains the domain name
			for _, ptr := range ptrs {
				if ptr != "" {
					return ptr
				}
			}
		}
	}
	_core.LogInfo("Domain name not found")
	return ""
}
func GetRuntimeArchBits() uint32 {
	const ptrSize = 32 << uintptr(^uintptr(0)>>63)
	if strconv.IntSize != ptrSize {
		_core.LogFatal("Could not accurately get runtime architecture bits")
	}
	return uint32(ptrSize)
}
func MoveFile(source string, destination string) {
	err := os.Rename(source, destination)
	if err != nil {
		_core.LogError("Could not move file: " + err.Error())
	}
}
func WriteFile(path string, data []byte) {
	err := os.WriteFile(path, data, 0644)
	if err != nil {
		_core.LogDebug("Could not write file: " + err.Error())
	}
}
func WriteEmbeddedFile(embedFS embed.FS, inputPath string, outputPath string) bool {
	data, err := fs.ReadFile(embedFS, inputPath)
	if err != nil {
		_core.LogError("Could not find embedded file: " + inputPath)
		return false
	}
	err = os.WriteFile(outputPath, data, 0644)
	if err != nil {
		_core.LogError("Could not write embedded file: " + outputPath)
		return false
	}
	return true
}
func WriteEmbeddedBinary(binary []byte, outputPath string) bool {
	//_core.LogDebug("Writing embedded binary to: " + outputPath)
	err := os.WriteFile(outputPath, binary, 0644)
	if err != nil {
		_core.LogWarn("Could not write embedded binary: " + outputPath + " - " + err.Error())
		return false
	}
	for i := 0; i < 5; i++ {
		time.Sleep(500 * time.Millisecond)
		if DoesExist(outputPath) {
			return true
		}
	}
	_core.LogWarn("Embedded binary not found after writing")
	return false
}
func WriteEmbeddedDir(embedFS embed.FS, inputPath string, outputPath string) bool {
	err := os.MkdirAll(outputPath, 0755)
	if err != nil {
		_core.LogError("Could not create embedded directory: " + outputPath)
		return false
	}
	err = fs.WalkDir(embedFS, inputPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			_core.LogError("Could not walk embedded directory: " + inputPath)
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(embedFS, path)
		if err != nil {
			_core.LogError("Could not read embedded file: " + path)
			return err
		}
		err = os.WriteFile(outputPath+PathSeparator+path, data, 0644)
		if err != nil {
			_core.LogError("Could not write embedded file: " + outputPath + PathSeparator + path)
			return err
		}
		return nil
	})
	if err != nil {
		_core.LogError("Could not walk embedded directory: " + inputPath)
		return false
	}
	return true
}
func Shutdown(code int) {
	_core.LogInfo("Shutting Down YourPlace Server")
	KillProcess("YourPlaceIpfs" + BinaryExtension)
	KillProcess("YourPlaceFfmpeg" + BinaryExtension)
	ReleaseMutex()
	proc, _ := os.FindProcess(int(GetSelfPID()))
	_ = proc.Signal(os.Interrupt)
	os.Exit(code)
}

/* ---------- Installers ---------- */
func PreInstall(favicon []byte) bool {
	if !DoesExist(GetInstallDir()) { // create install directory
		CreateFolder(GetInstallDir())
	}
	if !DoesExist(GetDataDir()) { // create user data directory
		CreateFolder(GetDataDir())
	}
	if !DoesExist(GetTempDir() + "YourPlaceIPFS" + PathSeparator) { // create IPFS temp download directory
		CreateFolder(GetTempDir() + "YourPlaceIPFS" + PathSeparator)
	}
	if !DoesExist(GetDataDir() + "upload") { // create default upload directory
		CreateFolder(GetDataDir() + "upload")
	}
	if !DoesExist(GetDataDir() + ".ipfs") { // create IPFS config directory
		CreateFolder(GetDataDir() + ".ipfs")
	}
	if !IsEmbeddedFileEqual(favicon, GetInstallDir()+"favicon.ico") { // install favicon
		err := os.WriteFile(GetInstallDir()+"favicon.ico", favicon, 0644)
		if err != nil {
			_core.LogWarn("Could not write the ico file: " + err.Error())
		}
	}
	if !InstallServerBinary() {
		_core.LogWarn("Could not install server binary")
	}
	if !InstallFFMPEG() {
		_core.LogWarn("Could not install FFMPEG")
	}
	if !InstallIPFS() {
		_core.LogWarn("Could not install IPFS")
	}
	if !InstallAutorun() {
		_core.LogWarn("Could not install YourPlace Autorun")
	}
	if !InstallHelper() {
		_core.LogWarn("Could not install YourPlace Helper")
	}
	return true
}
func InstallServerBinary() bool {
	currentBinary, err := os.Executable()
	if err != nil {
		_core.LogError("Could not get current binary path: " + err.Error())
		return false
	}
	destPath := GetInstallDir() + "YourPlace" + BinaryExtension
	if currentBinary == destPath { // If we're already in the installation path, no need to copy
		return false
	}
	DeleteIfExists(destPath)                // delete the old binary
	if !CopyFile(currentBinary, destPath) { // copy the binary to the installation directory
		_core.LogError("Could not copy server binary to install directory")
		return false
	}
	err = os.Chmod(destPath, 0755)
	if err != nil {
		_core.LogError("Could not set server binary permissions: " + err.Error())
		return false
	}
	if GetFileSize(destPath) == 0 {
		_core.LogError("Server binary file size is 0")
		return false
	}
	return true
}
func BackupData() bool {
	t := time.Now().Unix()
	outFile, err := os.Create(GetHomeDir() + PathSeparator + "YourPlace_Server_Backup_" + strconv.FormatInt(t, 10) + ".zip")
	if err != nil {
		_core.LogError("Could not create backup zip file: " + err.Error())
		return false
	}
	defer func(outFile *os.File) {
		err = outFile.Close()
		if err != nil {
			_core.LogError("Could not close backup zip file: " + err.Error())
			return
		}
	}(outFile)

	return false // debug
}
func UnInstall(keepUploads, keepBlockchain bool) bool {
	payload := "uninstall"
	if keepUploads {
		payload += " -keepUpload"
	}
	if keepBlockchain {
		payload += " -keepBlockchain"
	}
	response, err := HelperCall(payload)
	if err != nil {
		_core.LogError("Could not call helper: " + err.Error())
		return false
	}
	return response == "ok - uninstalling"
}
