package logging

type AnsiColor string

const (
	NO_COLOR AnsiColor = ""
	RESET    AnsiColor = "\033[0m"

	BLACK   AnsiColor = "\033[30m"
	RED     AnsiColor = "\033[31m"
	GREEN   AnsiColor = "\033[32m"
	YELLOW  AnsiColor = "\033[33m"
	BLUE    AnsiColor = "\033[34m"
	MAGENTA AnsiColor = "\033[35m"
	CYAN    AnsiColor = "\033[36m"
	WHITE   AnsiColor = "\033[37m"

	BRIGHT_RED    AnsiColor = "\033[91m"
	BRIGHT_GREEN  AnsiColor = "\033[92m"
	BRIGHT_YELLOW AnsiColor = "\033[93m"
	BRIGHT_BLUE   AnsiColor = "\033[94m"
)

var LogLevelColorMap = map[LogLevel]AnsiColor{
	LEVEL_DEBUG:   BRIGHT_BLUE,
	LEVEL_INFO:    BRIGHT_GREEN,
	LEVEL_WARNING: BRIGHT_YELLOW,
	LEVEL_ERROR:   BRIGHT_RED,
}

type Formatter interface {
	Format(record *LogRecord) []byte
}
