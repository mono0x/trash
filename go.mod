module github.com/mono0x/trash

go 1.26.10

require github.com/ebitengine/purego v0.11.2

require (
	github.com/BurntSushi/toml v1.4.1-0.20240526193622-a339e1f7089c // indirect
	github.com/kisielk/errcheck v1.30.0 // indirect
	golang.org/x/exp/typeparams v0.0.0-20231108232855-2478ac86f678 // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/telemetry v0.0.0-20261010193011-f71dc92da45e // indirect
	golang.org/x/tools v0.52.0 // indirect
	golang.org/x/vuln v1.9.0 // indirect
	honnef.co/go/tools v0.8.2 // indirect
)

tool (
	github.com/kisielk/errcheck
	golang.org/x/vuln/cmd/govulncheck
	honnef.co/go/tools/cmd/staticcheck
)
