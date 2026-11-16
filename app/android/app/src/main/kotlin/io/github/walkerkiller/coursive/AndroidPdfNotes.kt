package io.github.walkerkiller.coursive

import android.content.Context
import android.graphics.Bitmap
import android.system.Os
import com.tom_roush.pdfbox.android.PDFBoxResourceLoader
import com.tom_roush.pdfbox.cos.COSName
import com.tom_roush.pdfbox.io.MemoryUsageSetting
import com.tom_roush.pdfbox.pdmodel.PDDocument
import com.tom_roush.pdfbox.pdmodel.PDPage
import com.tom_roush.pdfbox.pdmodel.PDPageContentStream
import com.tom_roush.pdfbox.pdmodel.PDResources
import com.tom_roush.pdfbox.pdmodel.common.PDRectangle
import com.tom_roush.pdfbox.pdmodel.graphics.state.PDExtendedGraphicsState
import com.tom_roush.pdfbox.pdmodel.graphics.blend.BlendMode
import com.tom_roush.pdfbox.pdmodel.graphics.color.PDColor
import com.tom_roush.pdfbox.pdmodel.graphics.color.PDDeviceRGB
import com.tom_roush.pdfbox.pdmodel.interactive.annotation.PDAnnotationMarkup
import com.tom_roush.pdfbox.pdmodel.interactive.annotation.PDAppearanceDictionary
import com.tom_roush.pdfbox.pdmodel.interactive.annotation.PDAppearanceStream
import com.tom_roush.pdfbox.pdmodel.interactive.annotation.PDBorderStyleDictionary
import com.tom_roush.pdfbox.rendering.PDFRenderer
import io.flutter.plugin.common.MethodCall
import java.io.ByteArrayOutputStream
import java.io.File
import java.security.MessageDigest
import kotlin.math.max

/** Mirrors the working-copy metadata and displayed-page coordinates of the desktop editor. */
internal object AndroidPdfNotes {
    private const val ORIGIN = "CoursiveSourceSHA256"
    private const val MARGIN = "CoursiveRightMargin"
    private val saveLock = Any()

    // Long image decks must not keep a full compressed-file scratch buffer
    // per operation in the managed heap; PDFBox spills the excess to cache.
    private fun load(context: Context, file: File): PDDocument = PDDocument.load(
        file, MemoryUsageSetting.setupMixed(4L * 1024 * 1024).setTempDir(context.cacheDir))

    private fun hash(file: File): String {
        val digest = MessageDigest.getInstance("SHA-256")
        file.inputStream().buffered().use { input ->
            val bytes = ByteArray(65536)
            while (true) {
                val n = input.read(bytes)
                if (n < 0) break
                digest.update(bytes, 0, n)
            }
        }
        return digest.digest().joinToString("") { "%02x".format(it) }
    }

    private data class Geometry(val box: PDRectangle, val rotation: Int, val canExpand: Boolean) {
        val width = if (rotation % 180 == 0) box.width else box.height
        val height = if (rotation % 180 == 0) box.height else box.width
        fun extended(margin: Int): PDRectangle {
            val delta = width * margin / 100
            return PDRectangle(box.lowerLeftX - (if (rotation == 180) delta else 0f),
                box.lowerLeftY - (if (rotation == 270) delta else 0f),
                box.width + (if (rotation % 180 == 0) delta else 0f),
                box.height + (if (rotation % 180 != 0) delta else 0f))
        }
        fun point(x: Float, y: Float): Pair<Float, Float> {
            val (u, v) = when (rotation) {
                0 -> x to 1f-y
                90 -> y to x
                180 -> 1f-x to y
                else -> 1f-y to 1f-x
            }
            return box.lowerLeftX + u * box.width to box.lowerLeftY + v * box.height
        }
    }

    private fun geometry(page: PDPage): Geometry {
        val box = page.cropBox
        val media = page.mediaBox
        val equal = box.lowerLeftX == media.lowerLeftX && box.lowerLeftY == media.lowerLeftY &&
            box.upperRightX == media.upperRightX && box.upperRightY == media.upperRightY
        val rotation = (page.rotation % 360 + 360) % 360
        require(box.width > 0 && box.height > 0 && rotation in listOf(0,90,180,270))
        return Geometry(box, rotation, equal)
    }

    private fun managed(page: PDPage): List<Int> =
        page.annotations.mapIndexedNotNull { i, mark ->
            if (mark !is PDAnnotationMarkup || mark.subtype != "Ink") return@mapIndexedNotNull null
            if (mark.cosObject.getInt("CoursiveInk",0) == 1) i else null
        }

    private fun Geometry.normalized(x: Float, y: Float): List<Double> {
        val u=(x-box.lowerLeftX)/box.width
        val v=(y-box.lowerLeftY)/box.height
        val (a,b)=when(rotation){0->u to 1f-v; 90->v to u; 180->1f-u to v; else->1f-v to 1f-u}
        return listOf(a.toDouble(),b.toDouble())
    }

    private fun addInk(doc: PDDocument, page: PDPage, layout: Geometry, stroke: Map<*,*>, margin: Int) {
        val raw = stroke["points"] as List<*>
        require(raw.isNotEmpty())
        val points = raw.map {
            val pair = it as List<*>
            val x = (pair[0] as Number).toFloat()
            val y = (pair[1] as Number).toFloat()
            require(x in 0f..(1f + margin/100f) && y in 0f..1f)
            layout.point(x,y)
        }.toMutableList()
        if (points.all { it == points.first() }) points.add(points.first().let { it.first+.05f to it.second+.05f })
        val argb = (stroke["argb"] as? Number)?.toLong() ?: 0xff1f3b91L
        val width = (stroke["width"] as? Number)?.toFloat() ?: .003f
        require(width.isFinite() && width > 0 && argb in 0..0xffffffffL)
        val rgb = floatArrayOf(((argb shr 16) and 255)/255f, ((argb shr 8) and 255)/255f, (argb and 255)/255f)
        val square = stroke["square"] == true
        val marker = stroke["marker"] == true
        val line = layout.width * width
        val left = points.minOf { it.first } - line
        val bottom = points.minOf { it.second } - line
        val bounds = PDRectangle(left,bottom,points.maxOf { it.first } + line-left,points.maxOf { it.second } + line-bottom)
        val mark = PDAnnotationMarkup()
        mark.cosObject.setName(COSName.SUBTYPE, "Ink")
        mark.cosObject.setInt(COSName.getPDFName("CoursiveInk"),1)
        mark.rectangle = bounds
        mark.isPrinted = true
        mark.color = PDColor(rgb, PDDeviceRGB.INSTANCE)
        mark.cosObject.apply {
            setLong(COSName.getPDFName("CoursiveARGB"), argb)
            setFloat(COSName.getPDFName("CoursiveWidth"), width)
            setInt(COSName.getPDFName("CoursiveSquare"), if (square) 1 else 0)
            setInt(COSName.getPDFName("CoursiveMarker"), if (marker) 1 else 0)
        }
        mark.borderStyle = PDBorderStyleDictionary().apply { this.width = line }
        mark.inkList = arrayOf(points.flatMap { listOf(it.first, it.second) }.toFloatArray())
        val appearance = PDAppearanceStream(doc).apply {
            resources = PDResources()
            bBox = PDRectangle(bounds.width,bounds.height)
        }
        PDPageContentStream(doc, appearance).use { pen ->
            val style = PDExtendedGraphicsState().apply {
                strokingAlphaConstant = (argb shr 24)/255f
                if (marker) blendMode = BlendMode.DARKEN
            }
            pen.setGraphicsStateParameters(style)
            pen.setStrokingColor(rgb[0], rgb[1], rgb[2])
            pen.setLineWidth(line)
            pen.setLineCapStyle(if (square) 2 else 1)
            pen.setLineJoinStyle(1)
            pen.moveTo(points.first().first-left,points.first().second-bottom)
            for ((x,y) in points.drop(1)) pen.lineTo(x-left,y-bottom)
            pen.stroke()
        }
        mark.appearance = PDAppearanceDictionary().apply { setNormalAppearance(appearance) }
        page.annotations.add(mark)
    }

    fun run(context: Context, call: MethodCall): Any {
        PDFBoxResourceLoader.init(context.applicationContext)
        val source = File(call.argument<String>("source")!!).canonicalFile
        val op = call.argument<String>("op")
        if (op == "render") {
            val page = call.argument<Int>("page")!! - 1
            return load(context, source).use { doc ->
                val sheet = doc.getPage(page)
                val hidden = call.argument<Map<String,Any>>("edit")?.get("hidden") as? List<*>
                for (i in hidden.orEmpty().filterIsInstance<Int>().distinct().sortedDescending())
                    sheet.annotations.removeAt(i)
                val g = geometry(sheet)
                val bitmap = PDFRenderer(doc).renderImage(page, 1400f / max(g.width,g.height))
                try { ByteArrayOutputStream().use {
                    check(bitmap.compress(Bitmap.CompressFormat.PNG,100,it))
                    it.toByteArray()
                } } finally { bitmap.recycle() }
            }
        }
        val dest = File(call.argument<String>("output")!!).canonicalFile
        require(dest != source && dest.parentFile?.name == "courseware" &&
            dest.path.startsWith(context.filesDir.canonicalPath + File.separator))
        return synchronized(saveLock) {
            val layouts = load(context, source).use { doc ->
                require(!doc.isEncrypted && doc.numberOfPages in 1..5000)
                doc.pages.map { geometry(it) }
            }
            val origin = hash(source)
            val exists = dest.isFile
            val revision = if (exists) hash(dest) else ""
            load(context, if (exists) dest else source).use { doc ->
                val metadata = doc.documentInformation
                if (exists) require(metadata.getCustomMetadataValue(ORIGIN) == origin && layouts.size == doc.numberOfPages) {
                    "同名批注文件与原件不匹配"
                }
                val old = if (exists) metadata.getCustomMetadataValue(MARGIN)?.toInt() ?: 0 else 0
                if (op == "info") {
                    val ink = mutableListOf<Map<String,Any>>()
                    if (exists) doc.pages.forEachIndexed { n,page ->
                        for(i in managed(page)){
                            val mark = page.annotations[i] as PDAnnotationMarkup
                            for(line in mark.inkList){
                                val points = (0 until line.size-1 step 2).map { j -> layouts[n].normalized(line[j],line[j+1]) }
                                val attrs = mark.cosObject
                                ink.add(mapOf("page" to n+1,"index" to i,"points" to points,
                                    "argb" to attrs.getLong(COSName.getPDFName("CoursiveARGB"), 0xff1f3b91L),
                                    "width" to attrs.getFloat(COSName.getPDFName("CoursiveWidth"), .003f).toDouble(),
                                    "square" to (attrs.getInt(COSName.getPDFName("CoursiveSquare"), 0) == 1),
                                    "marker" to (attrs.getInt(COSName.getPDFName("CoursiveMarker"), 0) == 1)))
                            }
                        }
                    }
                    return@use mapOf("ink" to ink,"revision" to revision,"margin" to old,
                    "canExpand" to layouts.all { it.canExpand },
                    "sizes" to layouts.map { listOf(it.width.toDouble(),it.height.toDouble()) })
                }
                require(op == "save")
                val edit = call.argument<Map<String,Any>>("edit")!!
                require(edit["revision"] == revision) { "批注文件已被其他窗口修改" }
                val margin = edit["margin"] as Int
                require(margin in listOf(0,25,50) && margin >= old)
                if (margin != old) {
                    require(layouts.all { it.canExpand }) { "此文件有裁剪区域，无法安全扩边" }
                    doc.pages.forEachIndexed { i, page ->
                        val box = layouts[i].extended(margin)
                        page.mediaBox = box
                        page.cropBox = box
                    }
                }
                if (exists && edit["replaceInk"] == true) doc.pages.forEachIndexed { n, page ->
                    for (i in managed(page).asReversed()) page.annotations.removeAt(i)
                }
                for (raw in edit["strokes"] as List<*>) {
                    val stroke = raw as Map<*,*>
                    val i = (stroke["page"] as Int)-1
                    addInk(doc, doc.getPage(i), layouts[i], stroke, margin)
                }
                metadata.setCustomMetadataValue(ORIGIN,origin)
                metadata.setCustomMetadataValue(MARGIN,margin.toString())
                val tmp = File.createTempFile(".coursive-pdf-", ".tmp", dest.parentFile)
                try {
                    doc.save(tmp)
                    load(context, tmp).use { check(it.numberOfPages == layouts.size) }
                    val updated = hash(tmp)
                    if (Thread.currentThread().isInterrupted) throw InterruptedException("PDF save cancelled")
                    Os.rename(tmp.path,dest.path)
                    mapOf("revision" to updated)
                } finally { tmp.delete() }
            }
        }
    }
}
