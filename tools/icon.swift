// icon.swift draws the honk app icon: the goose emoji from the system font on a
// pond-colored squircle. `make app` runs it, so the repo holds no emoji art.
//
//   swift tools/icon.swift out.png
import AppKit

let size: CGFloat = 1024
let out = CommandLine.arguments.count > 1 ? CommandLine.arguments[1] : "icon.png"

guard let rep = NSBitmapImageRep(
    bitmapDataPlanes: nil, pixelsWide: Int(size), pixelsHigh: Int(size),
    bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
    colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)
else { fatalError("no bitmap") }

NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)

// Apple's icon grid: an 824 pt body with a 100 pt margin on each side.
let body = NSRect(x: 100, y: 100, width: 824, height: 824)
let shape = NSBezierPath(roundedRect: body, xRadius: 185, yRadius: 185)

// A drop shadow under the body, like the system icons.
NSGraphicsContext.saveGraphicsState()
let shadow = NSShadow()
shadow.shadowColor = NSColor.black.withAlphaComponent(0.3)
shadow.shadowOffset = NSSize(width: 0, height: -10)
shadow.shadowBlurRadius = 20
shadow.set()
NSColor(srgbRed: 0.10, green: 0.45, blue: 0.62, alpha: 1).setFill()
shape.fill()
NSGraphicsContext.restoreGraphicsState()

// Pond water: light at the top, deep at the bottom.
NSGradient(colors: [
    NSColor(srgbRed: 0.53, green: 0.86, blue: 0.93, alpha: 1),
    NSColor(srgbRed: 0.10, green: 0.45, blue: 0.62, alpha: 1),
])!.draw(in: shape, angle: -90)

// Two soft ripples behind the goose.
NSGraphicsContext.saveGraphicsState()
shape.addClip()
NSColor.white.withAlphaComponent(0.18).setStroke()
for (w, y) in [(560.0, 250.0), (400.0, 205.0)] {
    let r = NSBezierPath(ovalIn: NSRect(x: 512 - w / 2, y: y, width: w, height: w * 0.22))
    r.lineWidth = 14
    r.stroke()
}
NSGraphicsContext.restoreGraphicsState()

// The goose, with a soft shadow so it sits on the water.
NSGraphicsContext.saveGraphicsState()
let gooseShadow = NSShadow()
gooseShadow.shadowColor = NSColor.black.withAlphaComponent(0.35)
gooseShadow.shadowOffset = NSSize(width: 0, height: -14)
gooseShadow.shadowBlurRadius = 24
gooseShadow.set()
let goose = NSAttributedString(string: "🪿", attributes: [.font: NSFont(name: "Apple Color Emoji", size: 560)!])
let g = goose.size()
goose.draw(at: NSPoint(x: (size - g.width) / 2, y: (size - g.height) / 2 + 30))
NSGraphicsContext.restoreGraphicsState()

NSGraphicsContext.restoreGraphicsState()
guard let png = rep.representation(using: .png, properties: [:]) else { fatalError("no png") }
try! png.write(to: URL(fileURLWithPath: out))
