#!make

main_bin_name = rotector-slop
main_cmd_path = ./cmd/${main_bin_name}

tidy:
	go mod tidy
	go fmt ./...


clean:
	@if [ -d ./tmp ]; then rm -rf ./tmp; fi
	@if [ -d ./tmp/bin/${main_bin_name} ]; then rm -rf /tmp/bin/${main_bin_name}; fi

build: clean
	go build -o=./tmp/bin/${main_bin_name} ${main_cmd_path}
