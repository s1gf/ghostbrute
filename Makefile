TARGET=./dist
ARCHS=amd64 arm64
GOOS=windows linux darwin
PACKAGENAME="github.com/ghostbrute/ghostbrute"

COMMIT=`git rev-parse --short HEAD`
DATE=`date +%m/%d/%y`
GOVERSION=`go version | cut -d " " -f 3`

ifdef VERSION
	VERSION := $(VERSION)
else
	VERSION := dev
endif

LDFLAGS="-X ${PACKAGENAME}/util.GitCommit=${COMMIT} \
-X ${PACKAGENAME}/util.BuildDate=${DATE} \
-X ${PACKAGENAME}/util.GoVersion=${GOVERSION} \
-X ${PACKAGENAME}/util.Version=${VERSION} \
"

.PHONY: help windows linux mac all clean

help:           ## Show this help.
	@fgrep -h "##" $(MAKEFILE_LIST) | fgrep -v fgrep | sed -e 's/\\$$//' | sed -e 's/##//'

windows: ## Make Windows x86_64 and arm64 Binaries
	@for ARCH in ${ARCHS}; do \
		echo "Building for windows $${ARCH}.." ;\
		GOOS=windows GOARCH=$${ARCH} go build -a -ldflags ${LDFLAGS} -o ${TARGET}/ghostbrute_windows_$${ARCH}.exe || exit 1 ;\
	done; \
	echo "Done."

linux: ## Make Linux x86_64 and arm64 Binaries
	@for ARCH in ${ARCHS}; do \
		echo "Building for linux $${ARCH}..." ; \
		GOOS=linux GOARCH=$${ARCH} go build -a -ldflags ${LDFLAGS} -o ${TARGET}/ghostbrute_linux_$${ARCH} || exit 1 ;\
	done; \
	echo "Done."

mac: ## Make Darwin (Mac) x86_64 and arm64 Binaries
	@for ARCH in ${ARCHS}; do \
		echo "Building for mac $${ARCH}..." ; \
		GOOS=darwin GOARCH=$${ARCH} go build -a -ldflags ${LDFLAGS} -o ${TARGET}/ghostbrute_darwin_$${ARCH} || exit 1 ;\
	done; \
	echo "Done."

clean: ## Delete any binaries
	@rm -f ${TARGET}/* ; \
	go clean -i -n github.com/ghostbrute/ghostbrute ; \
	echo "Done."

all: ## Make Windows, Linux and Mac x86_64/arm64 Binaries
all: clean windows linux mac
