EXTERNAL_TOOLS=\
	github.com/mitchellh/gox \
	github.com/kardianos/govendor

# bootstrap the build by downloading additional tools
bootstrap:
	@for tool in  $(EXTERNAL_TOOLS) ; do \
		echo "Installing/Updating $$tool" ; \
		go get -u $$tool; \
	done
	govendor sync

full_build:
	gox -ldflags="-X main.GitCommit=$(git rev-parse HEAD) -X main.Version=$(git describe)"
