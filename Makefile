APP    := honk.app
LABEL   := dev.joeriddles.honk
DEST    := $(HOME)/Applications
AGENT   := $(HOME)/Library/LaunchAgents/$(LABEL).plist
# Flags for the app when it starts at login, for example:
#   make login ARGS="-exclude-orgs wildecare -team-honk"
ARGS    ?=

.PHONY: build test check sound app install login logout clean

build:
	go build -o honk .

test:
	go vet ./...
	go test ./...

check: build
	./honk -check $(ARGS)

sound: build
	./honk -test-sound

# A bundle with LSUIElement, so the app shows in the menubar and not in the Dock.
app: build
	rm -rf $(APP)
	mkdir -p $(APP)/Contents/MacOS
	cp honk $(APP)/Contents/MacOS/honk
	printf '%s\n' \
	  '<?xml version="1.0" encoding="UTF-8"?>' \
	  '<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">' \
	  '<plist version="1.0"><dict>' \
	  '<key>CFBundleExecutable</key><string>honk</string>' \
	  '<key>CFBundleIdentifier</key><string>$(LABEL)</string>' \
	  '<key>CFBundleName</key><string>honk</string>' \
	  '<key>CFBundlePackageType</key><string>APPL</string>' \
	  '<key>LSUIElement</key><true/>' \
	  '</dict></plist>' > $(APP)/Contents/Info.plist

install: app
	mkdir -p $(DEST)
	rm -rf $(DEST)/$(APP)
	cp -R $(APP) $(DEST)/
	@echo "Installed $(DEST)/$(APP). Start it with: open $(DEST)/$(APP)"

# Start honk at login with a LaunchAgent.
login: install
	mkdir -p $(dir $(AGENT))
	printf '%s\n' \
	  '<?xml version="1.0" encoding="UTF-8"?>' \
	  '<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">' \
	  '<plist version="1.0"><dict>' \
	  '<key>Label</key><string>$(LABEL)</string>' \
	  '<key>ProgramArguments</key><array><string>$(DEST)/$(APP)/Contents/MacOS/honk</string>$(foreach a,$(ARGS),<string>$(a)</string>)</array>' \
	  '<key>RunAtLoad</key><true/>' \
	  '<key>LimitLoadToSessionType</key><string>Aqua</string>' \
	  '</dict></plist>' > $(AGENT)
	launchctl bootout gui/$$(id -u) $(AGENT) 2>/dev/null || true
	launchctl bootstrap gui/$$(id -u) $(AGENT)
	@echo "honk now starts at login."

logout:
	launchctl bootout gui/$$(id -u) $(AGENT) 2>/dev/null || true
	rm -f $(AGENT)

clean:
	rm -rf honk $(APP)
