package gobonsai

import (
	"bytes"
	"errors"
	"io"
	"math"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
)

type AudioStream interface {
	io.ReadSeeker
	Length() int64
}

type AudiosManager struct {
	audioContext    *audio.Context
	audioFiles      map[string][]byte
	audioPlayers    map[string]*audio.Player
	logger          *Logger
	lastBeatNumbers map[string]int
}

func NewAudiosManager(sampleRate int) *AudiosManager {
	return &AudiosManager{
		audioContext: audio.NewContext(sampleRate),
		audioFiles:   make(map[string][]byte),
		audioPlayers: make(map[string]*audio.Player),
		logger:       NewLogger("bonsai:audios"),
	}
}

func (am *AudiosManager) AddAudio(name, path string) {
	if _, exists := am.audioFiles[name]; exists {
		am.logger.Warn("Audio file already loaded:", name)
		return
	}
	data := Membeds.GetFile(path)
	am.audioFiles[name] = data
	am.logger.Info("Audio file added:", name)
}

func (am *AudiosManager) AddAudios(audios map[string]string) {
	for name, path := range audios {
		am.AddAudio(name, path)
	}
}

func (am *AudiosManager) PlayAudio(name string, volume float64, loop bool) {
	if p, ok := am.audioPlayers[name]; ok {
		p.Rewind()
		p.SetVolume(volume)
		p.Play()
		am.logger.Info("Playing audio:", name, "Loop:", loop)
		return
	}
	data := am.audioFiles[name]
	if data == nil {
		am.logger.Warn("Audio data not found:", name)
		return
	}
	var player *audio.Player
	if loop {
		player = am.createLoopingPlayer(data)
	} else {
		player = am.createPlayer(data)
	}
	if player == nil {
		am.logger.Error("Failed to create audio player:", name)
		return
	}
	am.audioPlayers[name] = player
	player.SetVolume(volume)
	player.Play()
	am.logger.Info("Playing audio:", name, "Loop:", loop)
}

func (am *AudiosManager) StopAudio(name string) {
	p, ok := am.audioPlayers[name]
	if !ok {
		am.logger.Warn("Audio player not found:", name)
		return
	}
	p.Pause()
	p.Rewind()
	am.logger.Info("Stopped audio:", name)
}

func (am *AudiosManager) IsOnBeat(name string, bpm float64) bool {
	p, ok := am.audioPlayers[name]
	if !ok || !p.IsPlaying() {
		return false
	}

	if am.lastBeatNumbers == nil {
		am.lastBeatNumbers = make(map[string]int)
	}

	beatInterval := 60.0 / bpm
	position := p.Current().Seconds()

	currentBeatNumber := int(math.Floor(position / beatInterval))

	lastBeatNumber, exists := am.lastBeatNumbers[name]
	if !exists {
		lastBeatNumber = -1
	}

	isNewBeat := currentBeatNumber > lastBeatNumber

	if isNewBeat {
		am.lastBeatNumbers[name] = currentBeatNumber
	}

	return isNewBeat
}

func (am *AudiosManager) SetAudioVolume(name string, volume float64) {
	p, ok := am.audioPlayers[name]
	if !ok {
		am.logger.Warn("Audio player not found:", name)
		return
	}
	p.SetVolume(volume)
	am.logger.Info("Set volume:", name, volume)
}

func (am *AudiosManager) DisposeAudio() {
	for name, p := range am.audioPlayers {
		p.Close()
		delete(am.audioPlayers, name)
		am.logger.Info("Disposed audio player:", name)
	}
	am.audioFiles = make(map[string][]byte)
	am.logger.Info("Disposed all audio")
}

func (am *AudiosManager) decodeAudioData(data []byte) AudioStream {
	if len(data) < 4 {
		am.logger.Error("Invalid audio data")
		return nil
	}
	reader := bytes.NewReader(data)
	var (
		decoder AudioStream
		err     error
	)
	switch {
	case bytes.HasPrefix(data, []byte("RIFF")) && bytes.Contains(data[8:], []byte("WAVE")):
		decoder, err = wav.DecodeWithSampleRate(am.audioContext.SampleRate(), reader)
	case bytes.HasPrefix(data, []byte("OggS")):
		decoder, err = vorbis.DecodeWithSampleRate(am.audioContext.SampleRate(), reader)
	case bytes.HasPrefix(data, []byte("ID3")) || (data[0] == 0xFF && (data[1]&0xE0) == 0xE0):
		decoder, err = mp3.DecodeWithSampleRate(am.audioContext.SampleRate(), reader)
	default:
		err = errors.New("unsupported audio format")
	}
	if err != nil {
		am.logger.Error(err)
		return nil
	}
	return decoder
}

func (am *AudiosManager) createPlayer(data []byte) *audio.Player {
	d := am.decodeAudioData(data)
	if d == nil {
		return nil
	}
	p, err := am.audioContext.NewPlayer(d)
	if err != nil {
		am.logger.Error(err)
		return nil
	}
	return p
}

func (am *AudiosManager) createLoopingPlayer(data []byte) *audio.Player {
	d := am.decodeAudioData(data)
	if d == nil {
		return nil
	}
	loop := audio.NewInfiniteLoop(d, d.Length())
	p, err := am.audioContext.NewPlayer(loop)
	if err != nil {
		am.logger.Error(err)
		return nil
	}
	return p
}
