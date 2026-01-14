package ffmpeg

import (
	"fmt"
	"github.com/wieku/danser-go/app/settings"
	"github.com/wieku/danser-go/framework/files"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var ffmpegExec string

var output string

// check used encoders exist
func preCheck() {
	var err error

	ffmpegExec, err = files.GetCommandExec("ffmpeg", "ffmpeg")
	if err != nil {
		panic("ffmpeg not found! Please make sure it's installed in danser directory or in PATH. Follow download instructions at https://github.com/Wieku/danser-go/wiki/FFmpeg")
	}

	log.Println("FFmpeg exec location:", ffmpegExec)

	out, err := exec.Command(ffmpegExec, "-encoders").Output()
	if err != nil {
		if strings.Contains(err.Error(), "127") || strings.Contains(strings.ToLower(err.Error()), "0xc0000135") {
			panic(fmt.Sprintf("ffmpeg was installed incorrectly! Please make sure needed libraries (libs/*.so or bin/*.dll) are installed as well. Follow download instructions at https://github.com/Wieku/danser-go/wiki/FFmpeg. Error: %s", err))
		}

		panic(fmt.Sprintf("Failed to get encoder info. Error: %s", err))
	}

	encoders := strings.Split(string(out[:]), "\n")
	for i, v := range encoders {
		if strings.TrimSpace(v) == "------" {
			encoders = encoders[i+1 : len(encoders)-1]
			break
		}
	}

	vcodec := settings.Recording.Encoder
	acodec := settings.Recording.AudioCodec
	vfound := false
	afound := false

	for _, v := range encoders {
		encoder := strings.SplitN(strings.TrimSpace(v), " ", 3)
		codecType := string(encoder[0][0])

		if string(encoder[0][3]) == "X" {
			continue // experimental codec
		}

		if !vfound && codecType == "V" {
			vfound = encoder[1] == vcodec
		} else if !afound && codecType == "A" {
			afound = encoder[1] == acodec
		}
	}

	if !vfound {
		panic(fmt.Sprintf("Video codec %q does not exist", vcodec))
	}

	if !afound {
		panic(fmt.Sprintf("Audio codec %q does not exist", acodec))
	}
}

func StartFFmpeg(fps, _w, _h int, audioFPS float64, _output string) {
	preCheck()

	if strings.TrimSpace(_output) == "" {
		_output = "danser_" + time.Now().Format("2006-01-02_15-04-05")
	}

	output = _output

	log.Println("Starting encoding!")

	_ = os.RemoveAll(filepath.Join(settings.Recording.GetOutputDir(), output+"_temp"))

	err := os.MkdirAll(filepath.Join(settings.Recording.GetOutputDir(), output+"_temp"), 0755)
	if err != nil && !os.IsExist(err) {
		panic(err)
	}

	startVideo(fps, _w, _h)
	startAudio(audioFPS)
}

func StopFFmpeg() {
	log.Println("Finishing rendering...")

	stopVideo()
	stopAudio()

	log.Println("Ffmpeg finished.")

	if settings.Recording.SplitAudioTracks {
		moveSeparateAudioFiles()
	} else {
		combine()
	}
}

func combine() {
	options := []string{
		"-y",
		"-i", filepath.Join(settings.Recording.GetOutputDir(), output+"_temp", "video."+settings.Recording.Container),
		"-i", filepath.Join(settings.Recording.GetOutputDir(), output+"_temp", "audio."+settings.Recording.Container),
		"-c:v", "copy",
		"-c:a", "copy", "-strict", "-2",
	}

	if settings.Recording.Container == "mp4" {
		options = append(options, "-movflags", "+faststart")
	}

	finalOutputPath := filepath.Join(settings.Recording.GetOutputDir(), output+"."+settings.Recording.Container)

	options = append(options, finalOutputPath)

	log.Println("Starting composing audio and video into one file...")
	log.Println("Running ffmpeg with options:", options)
	cmd2 := exec.Command(ffmpegExec, options...)

	if settings.Recording.ShowFFmpegLogs {
		cmd2.Stdout = os.Stdout
		cmd2.Stderr = os.Stderr
	}

	if err := cmd2.Start(); err != nil {
		log.Println("Failed to start ffmpeg:", err)
	} else {
		if err = cmd2.Wait(); err != nil {
			panic(fmt.Sprintf("ffmpeg finished abruptly! Please check if you have enough storage. Error: %s", err))
		} else {
			log.Println("Finished!")
			log.Println("Video is available at:", finalOutputPath)
		}
	}

	cleanup()
}

func moveSeparateAudioFiles() {
	log.Println("Moving separate audio files to output directory...")
	
	tempDir := filepath.Join(settings.Recording.GetOutputDir(), output+"_temp")
	outputDir := settings.Recording.GetOutputDir()
	
	// 移动视频文件
	videoSrc := filepath.Join(tempDir, "video."+settings.Recording.Container)
	videoDst := filepath.Join(outputDir, output+"."+settings.Recording.Container)
	
	if err := os.Rename(videoSrc, videoDst); err != nil {
		log.Printf("Failed to move video file: %v", err)
	} else {
		log.Println("Video file moved to:", videoDst)
	}
	
	// 移动音乐文件
	musicSrc := filepath.Join(tempDir, "music.wav")
	musicDst := filepath.Join(outputDir, output+"_music.wav")
	
	if err := os.Rename(musicSrc, musicDst); err != nil {
		log.Printf("Failed to move music file: %v", err)
	} else {
		log.Println("Music file moved to:", musicDst)
	}
	
	// 移动音效文件
	soundSrc := filepath.Join(tempDir, "sound.wav")
	soundDst := filepath.Join(outputDir, output+"_sound.wav")
	
	if err := os.Rename(soundSrc, soundDst); err != nil {
		log.Printf("Failed to move sound file: %v", err)
	} else {
		log.Println("Sound file moved to:", soundDst)
	}
	
	log.Println("Separate audio files moved successfully!")
	log.Println("Video file:", videoDst)
	log.Println("Music file:", musicDst)
	log.Println("Sound file:", soundDst)
	
	cleanup()
}

func cleanup() {
	log.Println("Cleaning up intermediate files...")

	_ = os.RemoveAll(filepath.Join(settings.Recording.GetOutputDir(), output+"_temp"))

	log.Println("Finished.")
}
