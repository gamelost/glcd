FROM google/golang:latest

RUN curl -s https://s3.amazonaws.com/bitly-downloads/nsq/nsq-0.2.28.linux-amd64.go1.2.1.tar.gz > /root/nsq.tar.gz
RUN cd root/ && tar xvzf /root/nsq.tar.gz
RUN mv /root/nsq-0.2.28.linux-amd64.go1.2.1/bin/* /usr/local/bin/
RUN rmdir /root/nsq-0.2.28.linux-amd64.go1.2.1/bin /root/nsq-0.2.28.linux-amd64.go1.2.1
