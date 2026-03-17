package blades

import "strings"

// MIMEType 表示内容的媒体类型（MIME type）。
// MIMEType 用于标识文件、图像、音频、视频等内容的格式。
// 模型根据 MIMEType 决定如何处理多模态内容。
//
// 常见用途：
// 1. 标识 FilePart 和 DataPart 的内容类型
// 2. 帮助模型理解如何处理附件
// 3. 验证上传文件的格式
type MIMEType string

const (
	// 文本类型
	// MIMEText 纯文本格式
	MIMEText MIMEType = "text/plain"
	// MIMEMarkdown Markdown 格式
	MIMEMarkdown MIMEType = "text/markdown"

	// 常见图像类型
	// MIMEImagePNG PNG 图像格式
	MIMEImagePNG MIMEType = "image/png"
	// MIMEImageJPEG JPEG 图像格式
	MIMEImageJPEG MIMEType = "image/jpeg"
	// MIMEImageWEBP WEBP 图像格式
	MIMEImageWEBP MIMEType = "image/webp"

	// 常见音频类型（非穷举）
	// MIMEAudioWAV WAV 音频格式
	MIMEAudioWAV MIMEType = "audio/wav"
	// MIMEAudioMP3 MP3 音频格式
	MIMEAudioMP3 MIMEType = "audio/mpeg"
	// MIMEAudioOGG OGG 音频格式
	MIMEAudioOGG MIMEType = "audio/ogg"
	// MIMEAudioAAC AAC 音频格式
	MIMEAudioAAC MIMEType = "audio/aac"
	// MIMEAudioFLAC FLAC 无损音频格式
	MIMEAudioFLAC MIMEType = "audio/flac"
	// MIMEAudioOpus Opus 音频格式
	MIMEAudioOpus MIMEType = "audio/opus"
	// MIMEAudioPCM PCM 原始音频数据
	MIMEAudioPCM MIMEType = "audio/pcm"

	// 常见视频类型（非穷举）
	// MIMEVideoMP4 MP4 视频格式
	MIMEVideoMP4 MIMEType = "video/mp4"
	// MIMEVideoOGG OGG 视频格式
	MIMEVideoOGG MIMEType = "video/ogg"
)

// Type 返回 MIMEType 的一般类别。
// 返回值为 "image"、"audio"、"video" 或 "file"。
// 用于快速判断内容的大致类型。
//
// 示例：
//
//	MIMEImagePNG.Type() // "image"
//	MIMEAudioMP3.Type() // "audio"
//	MIMEText.Type()     // "file"
func (m MIMEType) Type() string {
	v := string(m)
	switch {
	case strings.HasPrefix(v, "image/"):
		return "image"
	case strings.HasPrefix(v, "audio/"):
		return "audio"
	case strings.HasPrefix(v, "video/"):
		return "video"
	default:
		return "file"
	}
}

// Format 返回 MIMEType 的文件格式部分。
// 格式是 MIMEType 中 "/" 后面的部分。
// 用于获取具体的文件扩展名或格式名称。
//
// 示例：
//
//	MIMEImagePNG.Format()   // "png"
//	MIMEAudioMP3.Format()   // "mpeg"
//	MIMEVideoMP4.Format()   // "mp4"
//	MIME("application/pdf").Format() // "pdf"
func (m MIMEType) Format() string {
	parts := strings.SplitN(string(m), "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return "octet-stream"
}
