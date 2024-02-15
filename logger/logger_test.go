package logger

import (
	"log"
	"testing"
)

func TestNewZeroLog(t *testing.T) {
	Init(-1, true)
	Logger.Info().Int("int", 1).Str("str", "str").Msg("test")
	log.Println("test")
}
