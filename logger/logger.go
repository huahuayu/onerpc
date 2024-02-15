package logger

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	stdlog "log"
	"os"
	"time"
)

var Logger zerolog.Logger

// adapter to make zerolog compatible with io.Writer interface
type logAdapter struct {
	logger zerolog.Logger
}

// Write implements io.Writer, allowing us to pass this adapter as the output for the std log package.
// This method will be called whenever the standard log package's functions are used.
func (a logAdapter) Write(p []byte) (n int, err error) {
	// Use zerolog to log the standard library log messages.
	a.logger.Info().Msg(string(p))
	return len(p), nil
}

func Init(level int, enableCaller bool) {
	// Convert the level from int to zerolog.Level
	// If the level is invalid, default to info
	zerologLevel := zerolog.InfoLevel
	if level >= int(zerolog.DebugLevel) && level <= int(zerolog.Disabled) {
		zerologLevel = zerolog.Level(level)
	}

	// Set the global time duration format for zerolog
	zerolog.DurationFieldUnit = time.Millisecond

	// Configure the logger
	cw := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
		FormatTimestamp: func(i interface{}) string {
			return "\033[34m" + i.(string) + "\033[0m"
		},
	}
	logger := zerolog.New(cw).Level(zerologLevel).With().Timestamp()

	if enableCaller {
		logger = logger.Caller()
	}

	// Set the configured logger as the global logger for zerolog
	log.Logger = logger.Logger()

	// Redirect standard log's output to the zerolog writer
	stdlog.SetOutput(logAdapter{logger.Logger()})
	stdlog.SetFlags(0) // Disable standard log's flags (like timestamp) since zerolog handles it

	Logger = logger.Logger()
}
