# 演示资源来源

- `images/sample.{png,jpg,gif,svg}`、`downloads/sample.{pdf,docx,zip,exe}`：本项目生成，MIT。生成源码在 `scripts/generate-fixtures.go`、`scripts/sample-download/main.go`。
- `fonts/SourceCodePro-Regular.{ttf,otf,woff,woff2}`：Adobe Source Code Pro，SIL Open Font License 1.1，原样分发，许可证见 `fonts/LICENSE.md`。来源：https://github.com/adobe-fonts/source-code-pro/tree/release 。字体仅用于字体加载测试，不替换应用系统字体。
- `media/flower.mp4`、`media/t-rex-roar.mp3`：MDN 示例媒体，原样分发。来源：https://mdn.github.io/shared-assets/videos/flower.mp4 与 https://mdn.github.io/shared-assets/audio/t-rex-roar.mp3 ，原示例位于 MDN interactive-examples 的 `media/cc0-videos`、`media/cc0-audio`；随附上游 `media/LICENSE.md`。
- `images/cherry.webp`：Benjamin Gimmel，A Wild Cherry (Prunus avium) in flower，经 Google WebP Gallery 提供的 WebP 版本，原样分发，CC BY-SA 3.0。来源：https://www.gstatic.com/webp/gallery/4.webp ，说明：https://developers.google.com/speed/webp/gallery1 ，许可证：https://creativecommons.org/licenses/by-sa/3.0/ 。此文件的许可证独立于项目 MIT 许可证。

- `media/roar-hls.mp3` 由上述 MDN 音频派生：生成器只替换 ID3 元数据，加入 HLS Packed Audio 时间戳，保留音频帧；`media/sample.m3u8` 为本项目生成的播放列表。

所有外站资源由本地受控虚拟站点提供，浏览器运行时不请求这些素材来源网站。
