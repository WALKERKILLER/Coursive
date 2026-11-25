# The device runtime exchanges these SDK data types by their public names.
-keep class com.huawei.stylus.penengine.estimate.** { *; }
-dontwarn com.huawei.penkit.**
-dontwarn com.huawei.stylus.penengine.version.**
-dontwarn com.huawei.stylus.penengine.view.IHwHandWritingView

# PDFBox treats the optional JPEG 2000 codec as a runtime capability.
-dontwarn com.gemalto.jp2.JP2Decoder
-dontwarn com.gemalto.jp2.JP2Encoder
