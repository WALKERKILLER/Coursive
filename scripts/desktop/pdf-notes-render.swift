// Render the same displayed crop and rotation used by the PDF annotation panel.
import AppKit
import PDFKit
guard CommandLine.arguments.count >= 3, let n = Int(CommandLine.arguments[2]), n > 0,
      let doc = PDFDocument(url: URL(fileURLWithPath: CommandLine.arguments[1])),
      n <= doc.pageCount, let page = doc.page(at: n-1) else { exit(1) }
if CommandLine.arguments.count > 3 {
    let annotations = page.annotations
    for i in CommandLine.arguments[3].split(separator: ",").compactMap({ Int($0) }) where annotations.indices.contains(i) {
        page.removeAnnotation(annotations[i])
    }
}
let box = page.bounds(for: .cropBox)
let sideways = page.rotation % 180 != 0
let width = sideways ? box.height : box.width, height = sideways ? box.width : box.height
guard width > 0, height > 0, width.isFinite, height.isFinite else { exit(1) }
let scale = min(2, 1500 / max(width,height))
let img = page.thumbnail(of: NSSize(width:width*scale,height:height*scale), for:.cropBox)
guard let tiff = img.tiffRepresentation, let rep = NSBitmapImageRep(data:tiff),
      let png = rep.representation(using:.png,properties:[:]) else { exit(1) }
FileHandle.standardOutput.write(png)
