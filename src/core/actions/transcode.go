package actions

import (
	"YourPlace/src/core/host"
	"path/filepath"
	"strings"
)

func FFMPEGTranscodeToHls(inputPath string, outputPath string) {
	hlsFileName := ParseHlsFileName(filepath.Base(inputPath))
	ffmpeg := host.GetFfmpegBin()
	ffmpegCommand := " -i " + inputPath + " -codec:v libx264 -start_number 0 -hls_time 10 -hls_list_size 0 -f hls " + outputPath + host.PathSeparator + hlsFileName
	host.RunShellCommand(ffmpeg + ffmpegCommand)
}
func ParseHlsFileName(oldFileName string) string {
	noExtFileName := strings.Split(oldFileName, ".")[0]
	hlsFileName := noExtFileName + ".m3u8"
	return hlsFileName
}
