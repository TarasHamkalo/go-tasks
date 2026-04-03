module downloader

go 1.25.8

// downgrade from v0.2.6 because of issue:
// https://github.com/c-bata/go-prompt/issues/228#issuecomment-818132118
require github.com/c-bata/go-prompt v0.2.5

require (
	github.com/google/uuid v1.6.0
	go.uber.org/zap v1.27.1
)

require (
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/mattn/go-tty v0.0.3 // indirect
	github.com/pkg/term v1.1.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/sys v0.0.0-20200918174421-af09f7315aff // indirect
)

replace github.com/c-bata/go-prompt => github.com/TarasHamkalo/go-prompt v0.2.5-exit
