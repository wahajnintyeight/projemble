module projemble

go 1.25.0

require (
	github.com/coreos/go-oidc/v3 v3.21.0
	github.com/metaspartan/gotui/v5 v5.0.3
	go.yaml.in/yaml/v3 v3.0.5
)

replace github.com/metaspartan/gotui/v5 => ./third_party/gotui

require (
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/gdamore/encoding v1.0.1 // indirect
	github.com/gdamore/tcell/v3 v3.4.2 // indirect
	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	github.com/mattn/go-runewidth v0.0.19 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	golang.org/x/image v0.34.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/term v0.45.0 // indirect
	golang.org/x/text v0.41.0 // indirect
)
