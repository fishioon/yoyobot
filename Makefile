BINARY=boardcode
VERSION=`git rev-parse --short HEAD`
BUILD=`date +%FT%T%z`

LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.Build=${BUILD}"

build:
	go build ${LDFLAGS}
install:
	go install ${LDFLAGS}
clean:
	if [ -f ${BINARY} ] ; then rm ${BINARY} ; fi

.PHONY:  clean install
