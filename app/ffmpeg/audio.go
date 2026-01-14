package ffmpeg

import (
	"fmt"
	"github.com/wieku/danser-go/app/settings"
	"github.com/wieku/danser-go/framework/bass"
	"github.com/wieku/danser-go/framework/files"
	"github.com/wieku/danser-go/framework/goroutines"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const MaxAudioBuffers = 2000

var cmdAudio, cmdMusicAudio, cmdEffectsAudio *exec.Cmd

var audioPipe, musicPipe, effectsPipe io.WriteCloser

var audioPool, musicPool, effectsPool chan []byte

var audioWriteQueue, musicWriteQueue, effectsWriteQueue chan []byte
var endSyncAudio, endSyncMusic, endSyncEffects *sync.WaitGroup

func startAudio(audioFPS float64) {
	if settings.Recording.SplitAudioTracks {
		startMusicAudio(audioFPS)
		startEffectsAudio(audioFPS)
	} else {
		startCombinedAudio(audioFPS)
	}
}

func startCombinedAudio(audioFPS float64) {
	inputName := "-"

	if runtime.GOOS != "windows" {
		pipe, err := files.NewNamedPipe("")
		if err != nil {
			panic(err)
		}

		inputName = pipe.Name()
		audioPipe = pipe
	}

	options := []string{
		"-y",

		"-f", "f32le",
		"-acodec", "pcm_f32le",
		"-ar", "48000",
		"-ac", "2",
		"-i", inputName,

		"-nostats", //hide audio encoding statistics because video ones are more important
		"-vn",
	}

	audioFilters := strings.TrimSpace(settings.Recording.AudioFilters)
	if len(audioFilters) > 0 {
		options = append(options, "-af", audioFilters)
	}

	options = append(options, "-c:a", settings.Recording.AudioCodec, "-strict", "-2")

	encOptions, err := settings.Recording.GetAudioOptions().GenerateFFmpegArgs()
	if err != nil {
		panic(fmt.Sprintf("encoder \"%s\": %s", settings.Recording.AudioCodec, err))
	} else if encOptions != nil {
		options = append(options, encOptions...)
	}

	options = append(options, filepath.Join(settings.Recording.GetOutputDir(), output+"_temp", "audio."+settings.Recording.Container))

	log.Println("Running ffmpeg with options:", options)

	cmdAudio = exec.Command(ffmpegExec, options...)

	if runtime.GOOS == "windows" {
		audioPipe, err = cmdAudio.StdinPipe()
		if err != nil {
			panic(err)
		}
	}

	if settings.Recording.ShowFFmpegLogs {
		cmdAudio.Stdout = os.Stdout
		cmdAudio.Stderr = os.Stderr
	}

	err = cmdAudio.Start()
	if err != nil {
		panic(fmt.Sprintf("ffmpeg's audio process failed to start! Please check if audio parameters are entered correctly or audio codec is supported by provided container. Error: %s", err))
	}

	audioBufSize := bass.GetMixerRequiredBufferSize(1 / audioFPS)

	audioPool = make(chan []byte, MaxAudioBuffers)

	for i := 0; i < MaxAudioBuffers; i++ {
		audioPool <- make([]byte, audioBufSize)
	}

	audioWriteQueue = make(chan []byte, MaxAudioBuffers)

	endSyncAudio = &sync.WaitGroup{}
	endSyncAudio.Add(1)

	goroutines.RunOS(func() {
		for data := range audioWriteQueue {
			if _, err := audioPipe.Write(data); err != nil {
				panic(fmt.Sprintf("ffmpeg's audio process finished abruptly! Please check if you have enough storage or audio parameters are entered correctly. Error: %s", err))
			}

			audioPool <- data
		}

		endSyncAudio.Done()
	})
}

func startMusicAudio(audioFPS float64) {
	inputName := "-"
	var err error

	if runtime.GOOS != "windows" {
		pipe, pipeErr := files.NewNamedPipe("")
		if pipeErr != nil {
			panic(pipeErr)
		}

		inputName = pipe.Name()
		musicPipe = pipe
	}

	// 输出为WAV格式，忽略压缩配置
	options := []string{
		"-y",

		"-f", "f32le",
		"-acodec", "pcm_f32le",
		"-ar", "48000",
		"-ac", "2",
		"-i", inputName,

		"-nostats",
		"-vn",
		"-c:a", "pcm_s16le", // WAV格式使用PCM编码
		"-ar", "48000",
	}

	options = append(options, filepath.Join(settings.Recording.GetOutputDir(), output+"_temp", "music.wav"))

	log.Println("Running ffmpeg for music with options:", options)

	cmdMusicAudio = exec.Command(ffmpegExec, options...)

	if runtime.GOOS == "windows" {
		musicPipe, err = cmdMusicAudio.StdinPipe()
		if err != nil {
			panic(err)
		}
	}

	if settings.Recording.ShowFFmpegLogs {
		cmdMusicAudio.Stdout = os.Stdout
		cmdMusicAudio.Stderr = os.Stderr
	}

	err = cmdMusicAudio.Start()
	if err != nil {
		panic(fmt.Sprintf("ffmpeg's music audio process failed to start! Error: %s", err))
	}

	audioBufSize := bass.GetMixerRequiredBufferSize(1 / audioFPS)

	musicPool = make(chan []byte, MaxAudioBuffers)

	for i := 0; i < MaxAudioBuffers; i++ {
		musicPool <- make([]byte, audioBufSize)
	}

	musicWriteQueue = make(chan []byte, MaxAudioBuffers)

	endSyncMusic = &sync.WaitGroup{}
	endSyncMusic.Add(1)

	goroutines.RunOS(func() {
		for data := range musicWriteQueue {
			if _, err := musicPipe.Write(data); err != nil {
				panic(fmt.Sprintf("ffmpeg's music audio process finished abruptly! Error: %s", err))
			}

			musicPool <- data
		}

		endSyncMusic.Done()
	})
}

func startEffectsAudio(audioFPS float64) {
	inputName := "-"
	var err error

	if runtime.GOOS != "windows" {
		pipe, pipeErr := files.NewNamedPipe("")
		if pipeErr != nil {
			panic(pipeErr)
		}

		inputName = pipe.Name()
		effectsPipe = pipe
	}

	// 输出为WAV格式，忽略压缩配置
	options := []string{
		"-y",

		"-f", "f32le",
		"-acodec", "pcm_f32le",
		"-ar", "48000",
		"-ac", "2",
		"-i", inputName,

		"-nostats",
		"-vn",
		"-c:a", "pcm_s16le", // WAV格式使用PCM编码
		"-ar", "48000",
	}

	options = append(options, filepath.Join(settings.Recording.GetOutputDir(), output+"_temp", "sound.wav"))

	log.Println("Running ffmpeg for sound effects with options:", options)

	cmdEffectsAudio = exec.Command(ffmpegExec, options...)

	if runtime.GOOS == "windows" {
		effectsPipe, err = cmdEffectsAudio.StdinPipe()
		if err != nil {
			panic(err)
		}
	}

	if settings.Recording.ShowFFmpegLogs {
		cmdEffectsAudio.Stdout = os.Stdout
		cmdEffectsAudio.Stderr = os.Stderr
	}

	err = cmdEffectsAudio.Start()
	if err != nil {
		panic(fmt.Sprintf("ffmpeg's effects audio process failed to start! Error: %s", err))
	}

	audioBufSize := bass.GetMixerRequiredBufferSize(1 / audioFPS)

	effectsPool = make(chan []byte, MaxAudioBuffers)

	for i := 0; i < MaxAudioBuffers; i++ {
		effectsPool <- make([]byte, audioBufSize)
	}

	effectsWriteQueue = make(chan []byte, MaxAudioBuffers)

	endSyncEffects = &sync.WaitGroup{}
	endSyncEffects.Add(1)

	goroutines.RunOS(func() {
		for data := range effectsWriteQueue {
			if _, err := effectsPipe.Write(data); err != nil {
				panic(fmt.Sprintf("ffmpeg's effects audio process finished abruptly! Error: %s", err))
			}

			effectsPool <- data
		}

		endSyncEffects.Done()
	})
}

func stopAudio() {
	if settings.Recording.SplitAudioTracks {
		stopMusicAudio()
		stopEffectsAudio()
	} else {
		stopCombinedAudio()
	}
}

func stopCombinedAudio() {
	log.Println("Audio finished! Stopping audio pipe...")

	close(audioWriteQueue)

	endSyncAudio.Wait()

	_ = audioPipe.Close()

	log.Println("Audio pipe closed. Waiting for audio ffmpeg process to finish...")

	_ = cmdAudio.Wait()

	log.Println("Audio process finished.")
}

func stopMusicAudio() {
	log.Println("Music audio finished! Stopping music pipe...")

	close(musicWriteQueue)

	endSyncMusic.Wait()

	_ = musicPipe.Close()

	log.Println("Music pipe closed. Waiting for music ffmpeg process to finish...")

	_ = cmdMusicAudio.Wait()

	log.Println("Music audio process finished.")
}

func stopEffectsAudio() {
	log.Println("Effects audio finished! Stopping effects pipe...")

	close(effectsWriteQueue)

	endSyncEffects.Wait()

	_ = effectsPipe.Close()

	log.Println("Effects pipe closed. Waiting for effects ffmpeg process to finish...")

	_ = cmdEffectsAudio.Wait()

	log.Println("Effects audio process finished.")
}

func PushAudio() {
	if settings.Recording.SplitAudioTracks {
		pushMusicAudio()
		pushEffectsAudio()
	} else {
		pushCombinedAudio()
	}
}

func pushCombinedAudio() {
	data := <-audioPool

	bass.ProcessMixer(data)

	audioWriteQueue <- data
}

func pushMusicAudio() {
	data := <-musicPool

	// 从音乐混合器获取数据
	bass.ProcessMusicMixer(data)

	musicWriteQueue <- data
}

func pushEffectsAudio() {
	data := <-effectsPool

	// 从音效混合器获取数据
	bass.ProcessEffectsMixer(data)

	effectsWriteQueue <- data
}
