NAME=shadowsocks-magic
BINDIR=bin
GOBUILD=CGO_ENABLED=0 go build -ldflags '-w -s -extldflags "-static"'
# The -w and -s flags reduce binary sizes by excluding unnecessary symbols and debug info

all: linux macos win64 arm mips

linux:
	GOARCH=amd64 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

macos:
	GOARCH=amd64 GOOS=darwin $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

win64:
	GOARCH=amd64 GOOS=windows $(GOBUILD) -o $(BINDIR)/$(NAME)-$@.exe

arm:
	GOARCH=arm GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

mips:
	GOARCH=mips GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

releases: linux macos win64 arm mips
	chmod +x $(BINDIR)/$(NAME)-*
	gzip $(BINDIR)/$(NAME)-linux
	gzip $(BINDIR)/$(NAME)-macos
	gzip $(BINDIR)/$(NAME)-arm
	gzip $(BINDIR)/$(NAME)-mips
	zip -m -j $(BINDIR)/$(NAME)-win64.zip $(BINDIR)/$(NAME)-win64.exe

clean:
	rm $(BINDIR)/*