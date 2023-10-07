build:
	go build -race main.go

clean-build:
	rm main
	go build -race main.go

broadcast-test:
	cd maelstrom; ./maelstrom test -w broadcast --bin ../main --node-count 1 --time-limit 20 --rate 10 || echo failure

unique-id:
	cd maelstrom; ./maelstrom test -w unique-ids --bin ../main --time-limit 30 --rate 1000 --node-count 3 --availability total --nemesis partition || echo failure