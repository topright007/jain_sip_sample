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
	"os"
	"os/signal"
	"syscall"
	//"unsafe"
)


func main()  {
	//ptr := C.malloc(C.sizeof_char * 1024)
	//defer C.free(unsafe.Pointer(ptr))
 	C.foo()
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
