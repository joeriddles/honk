APP    := honk.app
LABEL   := dev.joeriddles.honk
DEST    := $(HOME)/Applications
AGENT   := $(HOME)/Library/LaunchAgents/$(LABEL).plist

.PHONY: build test check sound app install login logout clean

build:
	go build -o honk .

test:
	go vet ./...
	go test ./...

check: build
	./honk -check

sound: build
	./honk -test-sound

# The app icon: the goose emoji from the system font, drawn by tools/icon.swift.
# It is made at build time, so the repo holds no emoji art.
honk.icns: tools/icon.swift
	rm -rf honk.iconset && mkdir honk.iconset
	swift tools/icon.swift honk.iconset/icon_512x512@2x.png
	for s in 16 32 128 256 512; do \
	  sips -z $$s $$s honk.iconset/icon_512x512@2x.png --out honk.iconset/icon_$${s}x$${s}.png >/dev/null; \
	  d=$$((s * 2)); sips -z $$d $$d honk.iconset/icon_512x512@2x.png --out honk.iconset/icon_$${s}x$${s}@2x.png >/dev/null; \
	done
	iconutil -c icns honk.iconset -o honk.icns
	rm -rf honk.iconset

# A bundle with LSUIElement, so the app shows in the menubar and not in the
# Dock. The ad-hoc signature gives macOS a stable identity for notifications.
app: build honk.icns
	rm -rf $(APP)
	mkdir -p $(APP)/Contents/MacOS $(APP)/Contents/Resources
	cp honk $(APP)/Contents/MacOS/honk
	cp honk.icns $(APP)/Contents/Resources/honk.icns
	printf '%s\n' \
	  '<?xml version="1.0" encoding="UTF-8"?>' \
	  '<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">' \
	  '<plist version="1.0"><dict>' \
	  '<key>CFBundleExecutable</key><string>honk</string>' \
	  '<key>CFBundleIdentifier</key><string>$(LABEL)</string>' \
	  '<key>CFBundleName</key><string>honk</string>' \
	  '<key>CFBundleDisplayName</key><string>honk</string>' \
	  '<key>CFBundleIconFile</key><string>honk</string>' \
	  '<key>CFBundlePackageType</key><string>APPL</string>' \
	  '<key>CFBundleShortVersionString</key><string>0.1.0</string>' \
	  '<key>CFBundleVersion</key><string>1</string>' \
	  '<key>LSMinimumSystemVersion</key><string>12.0</string>' \
	  '<key>LSUIElement</key><true/>' \
	  '</dict></plist>' > $(APP)/Contents/Info.plist
	codesign --force --sign - $(APP)

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
	  '<key>ProgramArguments</key><array><string>$(DEST)/$(APP)/Contents/MacOS/honk</string></array>' \
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
	rm -rf honk $(APP) honk.icns honk.iconset
