run:
	go run tftp/tftp.go --payload=./tftp/gopher.png

writerun:
	go run tftp/tftp.go --payload=./tftp/gopher.png --write


test: 
	go test ./...
