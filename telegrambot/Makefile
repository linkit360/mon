.PHONY: deploy


VERSION=$(shell git describe --always --long --dirty)

version:
	 @echo Version IS $(VERSION)

rm:
	rm ~/linkit/telegrambot-linux-amd64; rm -v bin/telegrambot-linux-amd64;

build:
	export GOOS=linux; export GOARCH=amd64; \
    go build -ldflags "-s -w" -o bin/telegrambot-linux-amd64 ; \
    cp  bin/telegrambot-linux-amd64 ~/linkit

deploy:
	# ssh -i ~/.ssh/monitoring_sg.pem centos@52.220.200.94 rm /home/centos/telegrambot/telegrambot-linux-amd64
	scp -i ~/.ssh/mobilink_centos_testing_stage.pem centos@35.154.8.158:/home/centos/vostrok/mon/telegrambot/bin/telegrambot-linux-amd64 .
	scp -i ~/.ssh/monitoring_sg.pem telegrambot-linux-amd64 centos@52.220.200.94:/home/centos/telegrambot/telegrambot-linux-amd64
