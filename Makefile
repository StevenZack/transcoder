run:
	replace -f internal/vars/mode.go "gin.DebugMode" "gin.ReleaseMode"
	GOOS=linux GOARCH=amd64 go build -o ~/release/transcoder 