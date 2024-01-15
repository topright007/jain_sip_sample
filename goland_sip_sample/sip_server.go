package main

import (
	"github.com/ghettovoice/gosip/sip"
	"github.com/pion/sdp"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/ghettovoice/gosip"
	"github.com/ghettovoice/gosip/log"
)

var (
	logger log.Logger
)

func init() {
	logger = log.NewDefaultLogrusLogger().WithPrefix("Server")
}

func getMidValue(media *sdp.MediaDescription) string {
	for _, attr := range media.Attributes {
		if attr.Key == "mid" {
			return attr.Value
		}
	}
	return ""
}

func onInvite(req sip.Request, tx sip.ServerTransaction) {
	toHeader, present := req.To()
	if !present {
		panic("to not present in request")
	}
	newCnt := &sip.ContactHeader{
		DisplayName: sip.String{Str: "the dude"},
		Address:     sip.NewAddressFromToHeader(toHeader).AsContactHeader().Address,
		Params:      sip.NewParams(),
	}
	//contactHeader := sip.ContactHeader{
	//	DisplayName: "some dude",
	//	Address: sip.ContactUri{ContoHeader.Address()),
	//	Params: sip.NewParams()
	//}
	var sd sdp.SessionDescription
	if err := sd.Unmarshal(req.Body()); err != nil {
		return
	}
	midValueCounter := 100
	for _, media := range sd.MediaDescriptions {
		midValue := getMidValue(media)
		if "" == midValue {
			midValueCounter += 1
			media.Attributes = append(media.Attributes, sdp.Attribute{Key: "mid", Value: strconv.Itoa(midValueCounter)})
		}
	}
	mungledOffer := sd.Marshal()
	logger.Info("Mungled offer ", mungledOffer)
	answer := connectFromOffer(mungledOffer)

	response := sip.NewResponseFromRequest(req.MessageID(), req, 200, "I said so", answer)
	response.AppendHeader(newCnt)
	response.Contact()
	err := tx.Respond(response)
	if err != nil {
		panic(err)
	}
}

func main() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	srvConf := gosip.ServerConfig{}
	srv := gosip.NewServer(srvConf, nil, nil, logger)

	if srv.OnRequest(sip.INVITE, onInvite) != nil {
		panic("Failed to register invite handler")
	}
	err := srv.Listen("ws", "0.0.0.0:5080", nil)
	if err != nil { panic(err) }
	//srv.Listen("wss", "0.0.0.0:5081", &transport.TLSConfig{Cert: "certs/cert.pem", Key: "certs/key.pem"})
	err = srv.Listen("udp", "0.0.0.0:5060", nil)
	if err != nil { panic(err) }

	logger.Info("SIP server Started")

	<-stop

	srv.Shutdown()
}
