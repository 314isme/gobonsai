package gobonsai

import (
	"bytes"
	"embed"
	"image"
	"os"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

type EmbedManager struct {
	embeddedFiles embed.FS
	logger        *Logger
}

func NewEmbedManager(embeds embed.FS) *EmbedManager {
	return &EmbedManager{
		embeddedFiles: embeds,
		logger:        NewLogger("bonsai:embed"),
	}
}

func (em *EmbedManager) GetFile(filePath string) []byte {
	var data []byte
	var err error

	if Debug {
		data, err = os.ReadFile(filePath)
	} else {
		filePath = strings.ReplaceAll(filePath, "\\", "/")
		data, err = em.embeddedFiles.ReadFile(filePath)
	}

	if err != nil {
		em.logger.Error("Failed to get embedded file:", filePath)
		return nil
	}
	return data
}

func (em *EmbedManager) GetFileAsImage(filePath string) *ebiten.Image {
	data := em.GetFile(filePath)
	if data == nil {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		em.logger.Error("Failed to decode image:", filePath)
		return nil
	}
	return ebiten.NewImageFromImage(img)
}
