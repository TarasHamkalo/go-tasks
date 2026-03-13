package logging

type AnsiColor string

const (
	NoColor AnsiColor = ""
	Reset   AnsiColor = "\033[0m"

	Black   AnsiColor = "\033[30m"
	Red     AnsiColor = "\033[31m"
	Green   AnsiColor = "\033[32m"
	Yellow  AnsiColor = "\033[33m"
	Blue    AnsiColor = "\033[34m"
	Magenta AnsiColor = "\033[35m"
	Cyan    AnsiColor = "\033[36m"
	White   AnsiColor = "\033[37m"

	BrightRed    AnsiColor = "\033[91m"
	BrightGreen  AnsiColor = "\033[92m"
	BrightYellow AnsiColor = "\033[93m"
	BrightBlue   AnsiColor = "\033[94m"
)

var LogLevelColorMap = map[LogLevel]AnsiColor{
	LevelDebug:   BrightBlue,
	LevelInfo:    BrightGreen,
	LevelWarning: BrightYellow,
	LevelError:   BrightRed,
}

type Formatter interface {
	Format(record *LogRecord) ([]byte, error)
}
