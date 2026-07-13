import AppKit

let args = CommandLine.arguments
guard args.count == 3 else {
    FileHandle.standardError.write("usage: swift seticon.swift <icon.icns> <target-file>\n".data(using: .utf8)!)
    exit(1)
}
guard let img = NSImage(contentsOfFile: args[1]) else {
    FileHandle.standardError.write("could not load icon: \(args[1])\n".data(using: .utf8)!)
    exit(1)
}
let ok = NSWorkspace.shared.setIcon(img, forFile: args[2], options: [])
exit(ok ? 0 : 2)
