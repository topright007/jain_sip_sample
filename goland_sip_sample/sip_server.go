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

// examples https://medium.com/ringcentral-developers/create-a-ringcentral-softphone-in-golang-7c4b7b079ed
// https://github.com/ringcentral/ringcentral-softphone-go

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

func mungleOffer(offer string) string {
	var sd sdp.SessionDescription
	if err := sd.Unmarshal(offer); err != nil {
		panic("failed to unmarshal offer " + offer)
	}
	midValueCounter := 100
	for _, media := range sd.MediaDescriptions {
		midValue := getMidValue(media)
		if "" == midValue {
			midValueCounter += 1
			media.Attributes = append(media.Attributes, sdp.Attribute{Key: "mid", Value: strconv.Itoa(midValueCounter)})
		}
		media.Attributes = append(media.Attributes, sdp.Attribute{Key: "sendrecv"} )
	}

	//some dummy fingerprint. validation will be disabled
	sd.Attributes = append(sd.Attributes, sdp.Attribute{Key: "fingerprint", Value: "fingerprint:sha-256 5D:8F:6B:D0:15:11:95:06:2E:AE:2B:C3:32:99:06:7C:2D:EA:D1:D1:AA:BF:07:D4:D3:16:32:61:53:30:EB:01"})
	mungledOffer := sd.Marshal()

	return mungledOffer
}

func mungleAnswer(answer string) string {
	var sd sdp.SessionDescription
	if err := sd.Unmarshal(answer); err != nil {
		panic("failed to unmarshal offer " + answer)
	}
	for _, media := range sd.MediaDescriptions {
		newAttrs := make([]sdp.Attribute, 0)
		for _, attr := range media.Attributes {
			if attr.Key != "mid" {
				newAttrs = append(newAttrs, attr)
			}
		}
		media.Attributes = newAttrs
	}
	sd.MediaDescriptions[0].MediaName.Protos = []string{"RTP", "SAVP"}

	//some dummy fingerprint. validation will be disabled
	mungledAnswer := sd.Marshal()

	return mungledAnswer
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
	mungledOffer := mungleOffer(req.Body())
	logger.Info("Mungled offer ", mungledOffer)
	answer := connectFromOffer(mungledOffer)

	answer = mungleAnswer(answer)
	logger.Info("Mungled answer ", answer)

	response := sip.NewResponseFromRequest(req.MessageID(), req, 200, "I said so", answer)
	response.AppendHeader(newCnt)
	response.Contact()
	err := tx.Respond(response)
	if err != nil {
		panic(err)
	}
}

func main() {
	err := os.Setenv("PION_LOG_", "all")
	if err != nil {panic(err)}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

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
