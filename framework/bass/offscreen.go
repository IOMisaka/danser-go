package bass

/*
#include <stdint.h>
#include "bass.h"
#include "bassmix.h"
*/
import "C"
import (
	"unsafe"
)

func GetMixerRequiredBufferSize(seconds float64) int {
	return int(C.BASS_ChannelSeconds2Bytes(masterMixer, C.double(seconds)))
}

func ProcessMixer(buffer []byte) {
	C.BASS_ChannelGetData(masterMixer, unsafe.Pointer(&buffer[0]), C.DWORD(len(buffer)))
}

func ProcessMusicMixer(buffer []byte) {
	if musicMixer != 0 {
		C.BASS_ChannelGetData(musicMixer, unsafe.Pointer(&buffer[0]), C.DWORD(len(buffer)))
	} else {
		// 如果没有独立的音乐混合器，从主混合器获取数据
		C.BASS_ChannelGetData(masterMixer, unsafe.Pointer(&buffer[0]), C.DWORD(len(buffer)))
	}
}

func ProcessEffectsMixer(buffer []byte) {
	// 音效总是从主混合器获取数据
	C.BASS_ChannelGetData(masterMixer, unsafe.Pointer(&buffer[0]), C.DWORD(len(buffer)))
}
