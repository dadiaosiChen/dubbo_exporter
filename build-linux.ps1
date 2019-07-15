$env:GOOS="linux"
$env:GOARCH="amd64"
go build -o ./bin/dubbo_exporter ./src/dubbo_exporter.go