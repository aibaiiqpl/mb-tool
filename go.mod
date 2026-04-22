module mb-tool

go 1.24.0

toolchain go1.24.11

require (
	aiwatt.net/ems/go-common v1.0.0
	github.com/goburrow/modbus v0.1.0
	github.com/tarm/serial v0.0.0-20180830185346-98f6abe2eb07
	go.uber.org/zap v1.27.1
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
)

replace aiwatt.net/ems/go-common v1.0.0 => cnb.cool/aiwatt/ems/go-common v1.0.0

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/goburrow/serial v0.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/stretchr/testify v1.9.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
)
