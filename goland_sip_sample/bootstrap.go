package main

// #cgo CFLAGS: -g -w
// #include <stdlib.h>
// #include <stdio.h>
// #include <libavcodec/avcodec.h>
//
import "C"

import (
	//"github.com/ghettovoice/gosip"
	//"github.com/ghettovoice/gosip/sip"
	//"os"
	//"os/signal"
	//"syscall"
	"github.com/ghettovoice/gosip"
	"github.com/ghettovoice/gosip/sip"
	"unsafe"

	//"image"
	//"image/color"
	//"image/draw"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	//"unsafe"
)

func main() {

	//f, err := os.Create("test.mpeg")
	//if err != nil {
	//	log.Panicf("Unable to open output file: %q", err)
	//}

	//im := image.NewRGBA(image.Rect(0, 0, 640, 480))

	f := C.fopen(C.CString("result.mpeg"), C.CString("w"))
	defer C.fclose(f)

	e, err := NewEncoder(CODEC_ID_H264, 640, 480)
	defer e.Close()

	if err != nil {
		log.Panicf("Unable to start encoder: %q", err)
	}

	start := time.Now()

	avPacket, freePacket := e.allocPacket()
	defer freePacket()

	for i := 0; i < 30*5; i++ {
		//c := color.RGBA{0, 0, uint8(i % 255), 255}
		// uint8(i%255), uint8(i%255), 255}
		//draw.Draw(im, im.Bounds(), &image.Uniform{c}, image.ZP, draw.Src)

		e.initPacket(avPacket, i)
		err, outSize := e.WriteFrame(avPacket)
		packet := (*C.AVPacket)(avPacket)
		if outSize >= 0 {
			C.fwrite(unsafe.Pointer(packet.data), C.ulong(packet.size), 1, f)
		}
		//f.Write()
		if err != nil {
			log.Panicf("Problem writing frame: %q", err)
		}
	}

	log.Printf("Took %s", time.Since(start))
}

func main1() {
	err := os.Setenv("PION_LOG_", "all")
	if err != nil {
		panic(err)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	go setupHttpServer()

	srvConf := gosip.ServerConfig{}
	srv := gosip.NewServer(srvConf, nil, nil, logger)

	if srv.OnRequest(sip.INVITE, onInvite) != nil {
		panic("Failed to register invite handler")
	}
	//err = srv.Listen("ws", "0.0.0.0:5080", nil)
	//if err != nil { panic(err) }
	//srv.Listen("wss", "0.0.0.0:5081", &transport.TLSConfig{Cert: "certs/cert.pem", Key: "certs/key.pem"})
	err = srv.Listen("udp", "0.0.0.0:5060", nil)
	if err != nil {
		panic(err)
	}

	logger.Info("SIP server Started")

	<-stop

	srv.Shutdown()
}
