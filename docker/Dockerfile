# start off with `go` installed
#
FROM dockerfile/go
MAINTAINER James Tranovich <jtranovich@gmail.com>

# install mercurial/bazaar since glcd dependencies require them:
#  snappy (from nsq) requires hg
#  mgo (from nsq) requires bzr
RUN apt-get update
RUN apt-get install -y --no-install-recommends mercurial
RUN apt-get install -y --no-install-recommends bzr

# install the glcd server
RUN go get github.com/gamelost/glcd

# configuration
WORKDIR /root
RUN mv /gopath/src/github.com/gamelost/glcd/glcd.config.default glcd.config
RUN /gopath/bin/glcd
