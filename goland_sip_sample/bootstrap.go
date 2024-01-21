package main

// //#cgo CFLAGS: -g -Wall
// #include <stdlib.h>
// #include <stdio.h>
//
// static inline void foo() {
//   fprintf(stderr, "** log\n");
//   printf("foo called\n");
// }
import "C"

import (
	//"github.com/ghettovoice/gosip"
	//"github.com/ghettovoice/gosip/sip"
	//"os"
	//"os/signal"
	//"syscall"
	"github.com/ghettovoice/gosip"
	"github.com/ghettovoice/gosip/sip"
	"image"
	"image/color"
	"image/draw"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	//"unsafe"
)


func main()  {

	f, err := os.Create("test.mpeg")
	if err != nil {
		log.Panicf("Unable to open output file: %q", err)
	}

	im := image.NewRGBA(image.Rect(0, 0, 640, 480))

	e, err := NewEncoder(CODEC_ID_H264, im, f)
	if err != nil {
		log.Panicf("Unable to start encoder: %q", err)
	}

	start := time.Now()

	for i := 0; i < 25*5; i++ {
		c := color.RGBA{0, 0, uint8(i % 255), 255}
		// uint8(i%255), uint8(i%255), 255}
		draw.Draw(im, im.Bounds(), &image.Uniform{c}, image.ZP, draw.Src)

		err := e.WriteFrame()
		if err != nil {
			log.Panicf("Problem writing frame: %q", err)
		}
	}

	e.Close()

	log.Printf("Took %s", time.Since(start))
}

func main1() {
	err := os.Setenv("PION_LOG_", "all")
	if err != nil {panic(err)}

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
	if err != nil { panic(err) }

	logger.Info("SIP server Started")

	<-stop

	srv.Shutdown()
}
