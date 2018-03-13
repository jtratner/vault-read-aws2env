EXTERNAL_TOOLS=\
	github.com/kardianos/govendor \

# bootstrap the build by downloading additional tools
bootstrap:
	@for tool in  $(EXTERNAL_TOOLS) ; do \
		echo "Installing/Updating $$tool" ; \
		go get -u $$tool; \
	done
	govendor sync
