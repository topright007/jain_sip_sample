package main

// #include <libavcodec/avcodec.h>
// #include <libswscale/swscale.h>
// #include <libavutil/imgutils.h>
//
// // ... yes. Don't ask.
// typedef struct SwsContext SwsContext;
//
// #ifndef PIX_FMT_RGB0
// #define PIX_FMT_RGB0 PIX_FMT_RGB32
// #endif
//
// #cgo pkg-config: libavdevice libavformat libavfilter libavcodec libswscale libavutil
import "C"
//sudo apt install libavdevice-dev

import (
	"errors"
	"fmt"
	//"image"
	//"io"
	"log"
	"reflect"
	"unsafe"
)


//example: https://github.com/pwaller/go-ffmpeg-video-encoding/blob/master/ffmpeg.go
//directly in ffmpeg: https://stackoverflow.com/questions/35569830/correctly-allocate-and-fill-frame-in-ffmpeg
//https://github.com/UnickSoft/FFmpeg-encode-example/blob/master/ffmpegEncoder/VideoEncoder.cpp#L121 - one more example
const (
	CODEC_ID_H264 = C.AV_CODEC_ID_H264
)

type Encoder struct {
	codec         uint32
	//im            image.Image
	//underlying_im image.Image
	//Output        io.Writer

	_codec      *C.AVCodec
	_context    *C.AVCodecContext
	_swscontext *C.SwsContext
	_frame      *C.AVFrame
	_outbuf     []byte
}


func init() {
	C.avcodec_register_all()
	C.av_log_set_level(C.AV_LOG_DEBUG)
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

func NewEncoder(codec uint32, width int, height int) (*Encoder, error) {
	_codec := C.avcodec_find_encoder(codec)
	if _codec == nil {
		return nil, fmt.Errorf("could not find codec")
	}

	avContext := C.avcodec_alloc_context3(_codec)
	avContext.bit_rate = 400000

	// resolution must be a multiple of two
	if width%2 == 1 || height%2 == 1 {
		return nil, fmt.Errorf("Bad image dimensions (%d, %d), must be even", width, height)
	}

	log.Printf("Encoder dimensions: %d, %d", width, height)

	avContext.width = C.int(width)
	avContext.height = C.int(height)
	avContext.time_base = C.AVRational{1, 30} // FPS
	avContext.gop_size = 10                   // emit one intra frame every ten frames
	avContext.max_b_frames = 0

	avContext.pix_fmt = C.AV_PIX_FMT_YUV420P
	avContext.bit_rate = C.long(400000)
	//avFrame.data[0] = ptr(underlying_im.Y)
	//avFrame.data[1] = ptr(underlying_im.Cb)
	//avFrame.data[2] = ptr(underlying_im.Cr)
	//avFrame.linesize[0] = w
	//avFrame.linesize[1] = w / 2
	//avFrame.linesize[2] = w / 2

	avFrame := C.av_frame_alloc()
	if avFrame == nil {
		panic("Failed to allicate avFrame")
	}
	avFrameData := [8]*C.uint8_t{}
	avFrameLinesizes := [8]C.int32_t{}
	C.av_image_alloc(&avFrameData[0], &avFrameLinesizes[0], avContext.width, avContext.height, C.AV_PIX_FMT_YUV420P, C.int(1))
	avFrame.format = C.AV_PIX_FMT_YUV420P
	avFrame.width = avContext.width
	avFrame.height = avContext.height
	avFrame.data = avFrameData
	avFrame.linesize = avFrameLinesizes

	size := C.avpicture_get_size(C.AV_PIX_FMT_YUV420P, avContext.width, avContext.height);
	picture_buf := (*C.uint8_t)(C.av_malloc(C.uint64_t(size)));
	if picture_buf == nil {
		C.av_free(unsafe.Pointer(avFrame))
		panic("Failed to allocate picture buf")
	}


	if C.avcodec_open2(avContext, _codec, nil) < 0 {
		return nil, fmt.Errorf("could not open codec")
	}

	_swscontext := C.sws_getContext(avContext.width, avContext.height, C.AV_PIX_FMT_RGB0, avContext.width, avContext.height, C.AV_PIX_FMT_YUV420P,
		C.SWS_BICUBIC, nil, nil, nil)

	e := &Encoder{
		codec,
		_codec,
		avContext,
		_swscontext,
		avFrame,
		make([]byte, 16*1024),
	}
	return e, nil
}

func (e *Encoder) WriteFrame() error {
	e._frame.pts = C.int64_t(e._context.frame_number)

	avPacket := C.av_packet_alloc()
	nSizeVideoEncodeBuffer := C.int(100000000)
	ret := C.av_new_packet(avPacket, nSizeVideoEncodeBuffer)
	if ret != 0 {
		panic(fmt.Sprintf("failed to init a packet %d", ret))
	}

	defer C.av_packet_unref(avPacket)



	//var input_data **C.uint8_t
	//var input_linesize [3]C.int

	//switch im := e.im.(type) {
	//case *image.RGBA:
	//	bpp := 4
	//	input_data = dptr(im.Pix)
	//	input_linesize = [3]C.int{C.int(e.im.Bounds().Dx() * bpp)}
	//case *image.NRGBA:
	//	bpp := 4
	//	input_data = dptr(im.Pix)
	//	input_linesize = [3]C.int{C.int(e.im.Bounds().Dx() * bpp)}
	//default:
	//	panic("Unknown input image type")
	//}

	// Perform scaling from input type to output type
	//C.sws_scale(
	//	e._swscontext,
	//	//nil,
	//	(input_data),
	//	&input_linesize[0],
	//	//&input_data[0],nil,
	//	//&input_linesize[0],
	//	0,
	//	e._context.height,
	//	&e._frame.data[0],
	//	&e._frame.linesize[0],
	//	)
	//C.sws_scale(e._swscontext, &input_data[0], &input_linesize[0],
	//	0, e._context.height,
	//	&e._frame.data[0], &e._frame.linesize[0])
	//print("", len(input_linesize), " ", input_data)
	//outsize := C.avcodec_encode_video(e._context, ptr(e._outbuf),
	//	C.int(len(e._outbuf)), e._frame)

	if err := doEncodeVideo(e, avPacket); err != nil {panic(err)}

	outsize := int(avPacket.size)
	logger.Info("encoded frame of size ", outsize)
	if outsize == 0 {
		return nil
	}

	//n, err := e.Output.Write(e._outbuf[:outsize])
	//if err != nil {
	//	return err
	//}
	//if n < int(outsize) {
	//	return fmt.Errorf("Short write, expected %d, wrote %d", outsize, n)
	//}

	return nil
}

func doEncodeVideo(e *Encoder, packet *C.AVPacket) error {
	//nSizeVideoEncodeBuffer := C.int(10000000)
	//var packet C.AVPacket
	//ret := C.av_new_packet(&packet, nSizeVideoEncodeBuffer)
	//if ret != 0 {
	//	panic(fmt.Sprintf("failed to init a packet %d", ret))
	//}
	////pVideoEncodeBuffer := (*C.uint8_t)(C.av_malloc(C.ulong(nSizeVideoEncodeBuffer)));
	//defer C.av_packet_unref(&packet)


	//packet.data = pVideoEncodeBuffer
	//packet.size = nSizeVideoEncodeBuffer

	//C.av_init_packet(packet)

	gotPacketStr := C.int(0)
	var successInt C.int = C.avcodec_encode_video2(
		e._context,
		packet,
		e._frame,
		&gotPacketStr,
	)

	if int(successInt) != 0 {
		return errors.New(fmt.Sprintf("failed to call avcodec_encode_video: %d. result length: %d", successInt, gotPacketStr))
	}
	logger.Info("successfully encoded frame. result len: ", gotPacketStr)

	return nil
}

func (e *Encoder) Close() {

	// Process "delayed" frames
	//for {
		//if err := doEncodeVideo(e); err != nil {panic(err)}
		//outsize := int(e._avPacket.size)
		//if outsize == 0 {
		//	break
		//}

		//n, err := e.Output.Write(e._outbuf[:outsize])
		//if err != nil {
		//	panic(err)
		//}
		//if n < int(outsize) {
		//	panic(fmt.Errorf("Short write, expected %d, wrote %d", outsize, n))
		//}
	//}

	//n, err := e.Output.Write([]byte{0, 0, 1, 0xb7})
	//if err != nil || n != 4 {
	//	log.Panicf("Error finishing mpeg file: %q; n = %d", err, n)
	//}

	C.avcodec_close((*C.AVCodecContext)(unsafe.Pointer(e._context)))
	C.av_free(unsafe.Pointer(e._context))
	C.av_free(unsafe.Pointer(e._frame.data[0]))
	C.av_free(unsafe.Pointer(e._frame))
	e._frame, e._codec = nil, nil
}