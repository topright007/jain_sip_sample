package main

import "C"
import (
	"fmt"
	"image"
	"image/color"
	"log"
	"reflect"
	"time"
	"unsafe"
)

// #cgo CFLAGS: -g -w
//#include <libavcodec/avcodec.h>
// #include <libswscale/swscale.h>
// #include <libavutil/imgutils.h>
// #include <libavutil/opt.h>
// #include "video_utils.h"
//
// // ... yes. Don't ask.
// typedef struct SwsContext SwsContext;
//
// #ifndef PIX_FMT_RGB0
// #define PIX_FMT_RGB0 PIX_FMT_RGB32
// #endif
//
//int FFMPEG_WAIT_FOR_INPUT_AVERROR = AVERROR(EAGAIN);
//
// #cgo pkg-config: libavdevice libavformat libavfilter libavcodec libswscale libavutil
import "C"

import (
	"errors"
)

// example: https://github.com/pwaller/go-ffmpeg-video-encoding/blob/master/ffmpeg.go
// directly in ffmpeg: https://stackoverflow.com/questions/35569830/correctly-allocate-and-fill-frame-in-ffmpeg
// https://github.com/UnickSoft/FFmpeg-encode-example/blob/master/ffmpegEncoder/VideoEncoder.cpp#L121 - one more example
const (
	CODEC_ID_H264 = C.AV_CODEC_ID_H264
)

type Encoder struct {
	codec uint32
	//im            image.Image
	//underlying_im image.Image
	//Output        io.Writer

	_codec      *C.AVCodec
	_context    *C.AVCodecContext
	_swscontext *C.SwsContext
	_frame      *C.AVFrame

	_outbuf     *C.uint8_t
	_outbuflen  C.int

	inputImage  *image.RGBA
	_input_data **C.uint8_t
	_input_linesize [1]C.int
}

type H264Packet *C.AVPacket

func (e *Encoder) allocPacket() (packet H264Packet, cancel func()) {
	avPacket := (*C.AVPacket)(packet)
	avPacket = C.av_packet_alloc()
	return avPacket, func() {
		C.av_packet_free(&avPacket)
	}
}

func yuvColor(y int, u int, v int) C.YUVColor {
	return C.yuv_color(C.uint8_t(y), C.uint8_t(u), C.uint8_t(v))
}

func drawBoxFrame(pic *C.AVFrame, x int, y int, width int, height int, color C.YUVColor) {
	drawBox((*C.AVPicture)(unsafe.Pointer(pic)), x, y, width, height, color)
}

func drawBox(pic *C.AVPicture, x int, y int, width int, height int, color C.YUVColor) {
	C.draw_box(pic, C.uint32_t(x), C.uint32_t(y), C.uint32_t(width), C.uint32_t(height), color)
}

func (e *Encoder) initPacket(packet H264Packet, frameNum int) {
	avPacket := (*C.AVPacket)(packet)
	C.av_init_packet(avPacket)
	avPacket.data = e._outbuf
	avPacket.size = e._outbuflen
	avPacket.pts = C.long(frameNum)
	avPacket.duration = 1
}

func initH264Encoder() {
	C.avcodec_register_all()
	C.av_log_set_level(C.AV_LOG_WARNING)
	//C.av_log_set_level(C.AV_LOG_DEBUG)
}

func ptr(buf []byte) *C.uint8_t {
	h := (*reflect.SliceHeader)(unsafe.Pointer(&buf))
	return (*C.uint8_t)(unsafe.Pointer(h.Data))
}

func dptr(buf []byte) **C.uint8_t {
	h := (*reflect.SliceHeader)(unsafe.Pointer(&buf))
	return (**C.uint8_t)(unsafe.Pointer(h.Data))
}

/*
type EncoderOptions struct {
    BitRate uint32
    W, H int
    TimeBase
} */

/*
var DefaultEncoderOptions = EncoderOptions{
    BitRate:400000,
    W: 0, H: 0,
    c.time_base = C.AVRational{1,25}
    c.gop_size = 10
    c.max_b_frames = 1
    c.pix_fmt = C.PIX_FMT_RGB
} */

type YCbCrWithSet struct {
	image.YCbCr
}

func (p *YCbCrWithSet) Set(x int, y int, c color.Color) {
	c1 := color.YCbCrModel.Convert(c).(color.YCbCr)
	p.Y[p.YOffset(x,y)] = c1.Y
	p.Cb[p.COffset(x,y)] = c1.Cb
	p.Cr[p.COffset(x,y)] = c1.Cr
}

func NewEncoder(codec uint32, inputImage *image.RGBA, fps int) (*Encoder, error) {
	_codec := C.avcodec_find_encoder(codec)
	if _codec == nil {
		return nil, fmt.Errorf("could not find codec")
	}

	width := inputImage.Bounds().Dx()
	height := inputImage.Bounds().Dy()

	// resolution must be a multiple of two
	if width%2 == 1 || height%2 == 1 {
		return nil, fmt.Errorf("Bad image dimensions (%d, %d), must be even", width, height)
	}
	log.Printf("Encoder dimensions: %d, %d", width, height)

	avContext := C.avcodec_alloc_context3(_codec)
	avContext.width = C.int(width)
	avContext.height = C.int(height)
	avContext.time_base = C.AVRational{1, C.int(fps)} // FPS
	avContext.gop_size = 100                   // emit one intra frame every ten frames
	avContext.max_b_frames = 0
	avContext.delay = 0

	avContext.pix_fmt = C.AV_PIX_FMT_YUV420P
	avContext.bit_rate = C.long(10485760)		//10 MBit

	C.av_opt_set(avContext.priv_data, C.CString("preset"), C.CString("ultrafast"), 0);
	C.av_opt_set(avContext.priv_data, C.CString("tune"), C.CString("zerolatency"), 0);

	avFrame := C.av_frame_alloc()
	if avFrame == nil {
		panic("Failed to allicate avFrame")
	}

	avFrame.format = C.AV_PIX_FMT_YUV420P
	avFrame.width = avContext.width
	avFrame.height = avContext.height

	//from AV
	avFrameData := [8]*C.uint8_t{}
	avFrameLinesizes := [8]C.int32_t{}
	C.av_image_alloc(&avFrameData[0], &avFrameLinesizes[0], avContext.width, avContext.height, C.AV_PIX_FMT_YUV420P, C.int(1))
	avFrame.data = avFrameData
	avFrame.linesize = avFrameLinesizes

	if C.avcodec_open2(avContext, _codec, nil) < 0 {
		return nil, fmt.Errorf("could not open codec")
	}

	_swscontext := C.sws_getContext(avContext.width, avContext.height, C.AV_PIX_FMT_RGB0, avContext.width, avContext.height, C.AV_PIX_FMT_YUV420P,
		C.SWS_BICUBIC, nil, nil, nil)

	videoEncodeBufLen := C.int(1024 * 1024)
	videoEncodeBuf := (*C.uint8_t)(C.av_malloc(C.ulong(videoEncodeBufLen)))


	input_data := (**C.uint8_t)(C.wrapWithArray(ptr(inputImage.Pix)))
	input_linesize := [1]C.int{C.int(inputImage.Bounds().Dx() * 4)}
	e := &Encoder{
		codec,
		_codec,
		avContext,
		_swscontext,
		avFrame,
		videoEncodeBuf,
		videoEncodeBufLen,
		inputImage,
		input_data,
		input_linesize,
	}
	return e, nil
}

func (e *Encoder) WriteFrame(avPacket H264Packet) (error, int) {
	e._frame.pts = C.int64_t(e._context.frame_number)

	err, outSize := doEncodeVideo(e, avPacket)
	if err != nil {
		panic(err)
	}

	return nil, outSize
}

func doEncodeVideo(e *Encoder, packet *C.AVPacket) (error, int) {
	start := time.Now()

	gotPacketStr := C.int(0)

	C.sws_scale(e._swscontext, e._input_data, &e._input_linesize[0],
		0, e._context.height,
		&e._frame.data[0], &e._frame.linesize[0])

	var successInt C.int = C.avcodec_send_frame(
		e._context,
		e._frame,
	)

	if int(successInt) != 0 {
		return errors.New(fmt.Sprintf(
				"failed to call avcodec_send_frame: %d. result length: %d",
				successInt,
				gotPacketStr,
			)),
			0
	}

	successInt = C.avcodec_receive_packet(
		e._context,
		packet,
	)

	if successInt == C.FFMPEG_WAIT_FOR_INPUT_AVERROR {
		logger.Info("Frame not encoded by libavicodec")
		return nil, -1
	}

	if int(successInt) != 0 {
		return errors.New(fmt.Sprintf(
				"failed to call avcodec_receive_packet: %d. result length: %d",
				successInt,
				gotPacketStr,
			)),
			0
	}

	logger.Info("successfully encoded frame."+
		"\npresentation ts: ", packet.pts,
		"\nstream index: ", packet.stream_index,
		"\npacket duration: ", packet.duration,
		"\nresult len: ", packet.size,
		"\nencoding took: ", time.Since(start),
	)

	return nil, int(packet.size)
}

func (e *Encoder) Close() {

	C.avcodec_close((*C.AVCodecContext)(unsafe.Pointer(e._context)))
	C.av_free(unsafe.Pointer(e._context))
	C.av_free(unsafe.Pointer(e._frame.data[0]))
	C.av_free(unsafe.Pointer(e._frame))
	C.av_free(unsafe.Pointer(e._outbuf))
	C.freeWrappedArray(e._input_data)
	e._frame, e._codec = nil, nil
}

func avPacketToSlice(avPacket H264Packet) []byte {
	packet := (*C.AVPacket)(avPacket)
	length := int(packet.size)
	var list []byte
	sliceHeader := (*reflect.SliceHeader)((unsafe.Pointer(&list)))
	sliceHeader.Cap = length
	sliceHeader.Len = length
	sliceHeader.Data = uintptr(unsafe.Pointer(packet.data))
	return list
}