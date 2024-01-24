package main

import "C"
import (
	"context"
	"fmt"
	"github.com/pion/interceptor"
	"github.com/pion/logging"
	"github.com/pion/sdp"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"image"
	"image/color"
	"image/draw"
	"os"
	"time"
)

var (
	RGBA_COLOR_RED = color.RGBA{200, 0, 0, 0xFF}
	RGBA_COLOR_BLACK = color.RGBA{0, 0, 0, 0xFF}
	RGBA_COLOR_WHITE = color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
	RGBA_COLOR_GRAD_LIGHT = color.RGBA{0xE8, 0xE6, 0xE2, 0xFF}
	RGBA_COLOR_GRAD_BLUE = color.RGBA{0xDC, 0xE5, 0xF7, 0xFF}
	RGBA_COLOR_ORANGE    = color.RGBA{0xff, 0x64, 0x27, 0xFF}
)

type VoiceMenuInstance struct {
	_peerConnection 		*webrtc.PeerConnection
	_iceConnectedCtx		context.Context
	_iceConnectedCtxCancel 	context.CancelFunc
	_videoTrack				*webrtc.TrackLocalStaticSample
	_videoTrackSender		*webrtc.RTPSender
	_encoder				*Encoder
}

func prepareSettingsEngine() webrtc.SettingEngine {
	settingEngine := webrtc.SettingEngine{}
	settingEngine.DisableCertificateFingerprintVerification(true)

	settingEngine.LoggerFactory = &logging.DefaultLoggerFactory{
		Writer:          os.Stdout,
		//DefaultLogLevel: logging.LogLevelTrace,
		DefaultLogLevel: logging.LogLevelWarn,
		ScopeLevels: map[string]logging.LogLevel{
			"ice": logging.LogLevelWarn,
			//"ice": logging.LogLevelDebug,
		},
	}
	return settingEngine
}

func prepareMediaEngine() *webrtc.MediaEngine{
	mediaEngine :=&webrtc.MediaEngine{}
	if err := mediaEngine.RegisterDefaultCodecs(); err != nil { panic(err) }
	return mediaEngine
}

func prepareWebRTCInterceptors(mediaEngine *webrtc.MediaEngine) *interceptor.Registry {
	interceptors := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(mediaEngine, interceptors); err != nil { panic(err) }
	return interceptors
}

func preparePeerConnection(api *webrtc.API, iceConnectedCtxCancel context.CancelFunc, candidatesChannel chan string) *webrtc.PeerConnection {
	peerConnection, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{
					//"stun:stun.l.google.com:19302"
				},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
		if connectionState == webrtc.ICEConnectionStateConnected {
			//sleep for 1 sec to wait for the connection to be established
			time.Sleep(time.Second * time.Duration(2))
			iceConnectedCtxCancel()
		}
	})

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		fmt.Printf("Peer Connection State has changed: %s\n", s.String())

		if s == webrtc.PeerConnectionStateFailed {
			// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
			// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
			// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
			fmt.Println("Peer Connection has gone to failed exiting")
		}

		if s == webrtc.PeerConnectionStateClosed {
			// PeerConnection was explicitly closed. This usually happens from a DTLS CloseNotify
			fmt.Println("Peer Connection has gone to closed")
		}
	})

	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			candidatesChannel <- ""
			return
		}
		candidatesChannel <- fmt.Sprintf("%s %d %s %d %s %d typ %s",
			candidate.Foundation,
			candidate.Component,
			candidate.Protocol.String(),
			candidate.Priority,
			candidate.Address,
			candidate.Port,
			candidate.Typ)
	})

	return peerConnection
}

func (vmi *VoiceMenuInstance) prepareEncoder () {
	e, err := NewEncoder(CODEC_ID_H264, image.NewRGBA(image.Rect(0,0,1280,720)))
	if err != nil {panic(err)}
	vmi._encoder = e
}

func (vmi *VoiceMenuInstance) connect(offerStr string) string {
	iceConnectedCtx, iceConnectedCtxCancel := context.WithCancel(context.Background())
	vmi._iceConnectedCtx = iceConnectedCtx
	vmi._iceConnectedCtxCancel = iceConnectedCtxCancel
	candidatesChannel := make(chan string)

	settingEngine := prepareSettingsEngine()
	mediaEngine := prepareMediaEngine()
	interceptors := prepareWebRTCInterceptors(mediaEngine)

	apiWithSettings := webrtc.NewAPI(
		webrtc.WithSettingEngine(settingEngine),
		webrtc.WithMediaEngine(mediaEngine),
		webrtc.WithInterceptorRegistry(interceptors),
	)

	vmi._peerConnection = preparePeerConnection(apiWithSettings, iceConnectedCtxCancel, candidatesChannel)

	videoTrack, videoTrackErr := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 30},
		"video",
		"pion",
		)
	if videoTrackErr != nil {
		panic(videoTrackErr)
	}

	rtpSender, videoTrackErr := vmi._peerConnection.AddTrack(videoTrack)
	if videoTrackErr != nil {
		panic(videoTrackErr)
	}

	vmi._videoTrack = videoTrack
	vmi._videoTrackSender = rtpSender

	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offerStr,
	}
	//todo: read the offer
	//signal.Decode(signal.MustReadStdin(), &offer)

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(vmi._peerConnection)

	// Set the remote SessionDescription
	if err := vmi._peerConnection.SetRemoteDescription(offer); err != nil {
		panic(err)
	}

	answer, err := vmi._peerConnection.CreateAnswer(&webrtc.AnswerOptions{})
	if err != nil {
		panic(err)
	}
	if err = vmi._peerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	answerSD := sdp.SessionDescription{}
	if err = answerSD.Unmarshal(answer.SDP); err != nil{ panic(err) }

	for cand := range(candidatesChannel) {
		if cand == "" {
			break
		}
		for _, md := range(answerSD.MediaDescriptions) {
			md.Attributes = append(md.Attributes, sdp.Attribute{Key: "candidate", Value: cand})
		}
	}

	close(candidatesChannel)

	vmi.prepareEncoder()

	<-gatherComplete
	logger.Info("Gathering complete. Answer set as local description\n" + answer.SDP)

	return answerSD.Marshal()
}

func (vmi *VoiceMenuInstance) StartVideoPlayback() {
	<-vmi._iceConnectedCtx.Done()

	numerator := 1
	denominator := 10
	videoDurationBetweenFrames := (float32(numerator)/float32(denominator))*1000
	logger.Info("Peer connection established. sending video. Between frames: ",
		videoDurationBetweenFrames,
		" milliseconds",
	)

	avPacket, freePacket := vmi._encoder.allocPacket()
	defer freePacket()

	ticker := time.NewTicker(time.Millisecond * time.Duration(videoDurationBetweenFrames))
	for i:= 0; true; i++  {
		<-ticker.C
		outSize := -1
		var err error
		//sometimes ffmpeg skips frames
		for outSize < 0 {
			vmi._encoder.initPacket(avPacket, i)

			inputImage := vmi._encoder.inputImage

			xShift := 2*i % (inputImage.Bounds().Dx() - 200) + 100

			draw.Draw(inputImage, inputImage.Bounds(), &image.Uniform{RGBA_COLOR_GRAD_LIGHT}, image.Point{}, draw.Src)
			draw.Draw(inputImage, image.Rect(xShift, 110, 100+xShift, 150), &image.Uniform{RGBA_COLOR_ORANGE}, image.Point{}, draw.Src)
			addLabel(inputImage, xShift, 100, "heyhey!!! DTMF coming soon!!!")

			err, outSize = vmi._encoder.WriteFrame(avPacket)
			if err != nil {panic(err)}
		}
		packetSlice := avPacketToSlice(avPacket)

		if ivfErr := vmi._videoTrack.WriteSample(media.Sample{Data: packetSlice, Duration: time.Second}); ivfErr != nil {
			panic(ivfErr)
		}
	}
}