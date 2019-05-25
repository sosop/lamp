APP = lamp_2.0.0

build:
	go build -o ./bin/${APP} -ldflags '-s -w'

linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./bin/${APP} -ldflags '-s -w'

run:
	@go run *.go

clean:
	@rm ./bin/${APP}
