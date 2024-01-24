package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/pion/interceptor"
	"github.com/pion/logging"
	"github.com/pion/sdp"
	"io"
	"os"
	"time"

	"github.com/pion/webrtc/v4"
	//"github.com/pion/webrtc/v4/examples/internal/signal"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/ivfreader"
	"github.com/pion/webrtc/v4/pkg/media/oggreader"
)

// https://github.com/pion/webrtc/blob/master/examples/play-from-disk/main.go
// generate video:
// generate audio: ffmpeg -f lavfi -i "sine=frequency=1000:sample_rate=48000:duration=60" \
// -i "sine=frequency=400:sample_rate=48000:duration=60" \
// -filter_complex amerge -c:a libopus -page_duration 20000 -vn testsrc.ogg
const (
	audioFileName   = "/home/topright/tmp/testvid/testsrc.ogg"
	videoFileName   = "/home/topright/tmp/testvid/testsrc.ivf"
)

func startAudioPlayback(audioTrack *webrtc.TrackLocalStaticSample) {
	func() {
		// Open a OGG file and start reading using our OGGReader
		file, oggErr := os.Open(audioFileName)
		if oggErr != nil {
			panic(oggErr)
		}

		// Open on oggfile in non-checksum mode.
		ogg, _, oggErr := oggreader.NewWith(file)
		if oggErr != nil {
			panic(oggErr)
		}


		// Keep track of last granule, the difference is the amount of samples in the buffer
		var lastGranule uint64

		// It is important to use a time.Ticker instead of time.Sleep because
		// * avoids accumulating skew, just calling time.Sleep didn't compensate for the time spent parsing the data
		// * works around latency issues with Sleep (see https://github.com/golang/go/issues/44343)

		//time.Sleep(time.Duration(10) * time.Second)

		ticker := time.NewTicker(audioOggPageDuration)
		for ; true; <-ticker.C {
			pageData, pageHeader, oggErr := ogg.ParseNextPage()
			if errors.Is(oggErr, io.EOF) {
				fmt.Printf("All audio pages parsed and sent")
				break
				//os.Exit(0)
			}

			if oggErr != nil {
				panic(oggErr)
			}

			// The amount of samples is the difference between the last and current timestamp
			sampleCount := float64(pageHeader.GranulePosition - lastGranule)
			lastGranule = pageHeader.GranulePosition
			sampleDuration := time.Duration((sampleCount/48000)*1000) * time.Millisecond

			if oggErr = audioTrack.WriteSample(media.Sample{Data: pageData, Duration: sampleDuration}); oggErr != nil {
				panic(oggErr)
			}
		}
	}()
}

func startVideoPlayback(videoTrack *webrtc.TrackLocalStaticSample) {
	// Open a IVF file and start reading using our IVFReader
	logger.Info("Openning video file " + videoFileName)
	file, ivfErr := os.Open(videoFileName)
	if ivfErr != nil {
		panic(ivfErr)
	}

	ivf, header, ivfErr := ivfreader.NewWith(file)
	if ivfErr != nil {
		panic(ivfErr)
	}

	videoDurationBetweenFrames := (float32(header.TimebaseNumerator)/float32(header.TimebaseDenominator))*1000
	logger.Info("Peer connection established. sending video of duration ",
		videoDurationBetweenFrames * float32(header.NumFrames),
		" seconds. Between frames: ",
		videoDurationBetweenFrames,
		" milliseconds",
	)

	// Send our video file frame at a time. Pace our sending so we send it at the same speed it should be played back as.
	// This isn't required since the video is timestamped, but we will such much higher loss if we send all at once.
	//
	// It is important to use a time.Ticker instead of time.Sleep because
	// * avoids accumulating skew, just calling time.Sleep didn't compensate for the time spent parsing the data
	// * works around latency issues with Sleep (see https://github.com/golang/go/issues/44343)

	ticker := time.NewTicker(time.Millisecond * time.Duration(videoDurationBetweenFrames))
	for ; true; <-ticker.C {
		frame, _, ivfErr := ivf.ParseNextFrame()
		if errors.Is(ivfErr, io.EOF) {
			fmt.Printf("All video frames parsed and sent")
			break
		}

		if ivfErr != nil {
			panic(ivfErr)
		}

		if ivfErr = videoTrack.WriteSample(media.Sample{Data: frame, Duration: time.Second}); ivfErr != nil {
			panic(ivfErr)
		}
	}
}

func startVideoRender(videoTrack *webrtc.TrackLocalStaticSample) {
	// Open a IVF file and start reading using our IVFReader
	logger.Info("Openning video file " + videoFileName)

	fps := 30
	videoDurationBetweenFrames := float32(1000) / float32(fps)
	logger.Info("Generating test video. Between frames: ",
		videoDurationBetweenFrames,
		" milliseconds",
	)

	// Send our video file frame at a time. Pace our sending so we send it at the same speed it should be played back as.
	// This isn't required since the video is timestamped, but we will such much higher loss if we send all at once.
	//
	// It is important to use a time.Ticker instead of time.Sleep because
	// * avoids accumulating skew, just calling time.Sleep didn't compensate for the time spent parsing the data
	// * works around latency issues with Sleep (see https://github.com/golang/go/issues/44343)

	ticker := time.NewTicker(time.Millisecond * time.Duration(videoDurationBetweenFrames))
	for ; true; <-ticker.C {
		frame := make([]byte, 10500)
		//frame, _, ivfErr := ivf.ParseNextFrame()

		if err := videoTrack.WriteSample(media.Sample{Data: frame, Duration: time.Second}); err != nil {
			panic(err)
		}
	}
}


// nolint:gocognit
func connectFromOffer(offerStr string) string {
	// Assert that we have an audio or video file
	_, err := os.Stat(videoFileName)
	haveVideoFile := !os.IsNotExist(err)

	_, err = os.Stat(audioFileName)
	haveAudioFile := !os.IsNotExist(err)

	if !haveAudioFile && !haveVideoFile {
		panic("Could not find `" + audioFileName + "` or `" + videoFileName + "`")
	}

	settingEngine := webrtc.SettingEngine{}
	settingEngine.DisableCertificateFingerprintVerification(true)

	settingEngine.LoggerFactory = &logging.DefaultLoggerFactory{
		Writer:          os.Stdout,
		DefaultLogLevel: logging.LogLevelTrace,
		ScopeLevels: map[string]logging.LogLevel{
			"ice": logging.LogLevelDebug,
		},
	}

	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterDefaultCodecs(); err != nil { panic(err) }

	interceptors := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(mediaEngine, interceptors); err != nil { panic(err) }

	apiWithSettings := webrtc.NewAPI(
		webrtc.WithSettingEngine(settingEngine),
		webrtc.WithMediaEngine(mediaEngine),
		webrtc.WithInterceptorRegistry(interceptors),
	)

	// Create a new RTCPeerConnection
	peerConnection, err := apiWithSettings.NewPeerConnection(webrtc.Configuration{
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

	//if _, err = peerConnection.CreateDataChannel("audio", nil) ; err != nil {panic(err)}
	//if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {panic(err)}

	//defer func() {
	//	if cErr := peerConnection.Close(); cErr != nil {
	//		fmt.Printf("cannot close peerConnection: %v\n", cErr)
	//	}
	//}()

	iceConnectedCtx, iceConnectedCtxCancel := context.WithCancel(context.Background())

	if haveVideoFile {
		file, openErr := os.Open(videoFileName)
		if openErr != nil {
			panic(openErr)
		}

		_, header, openErr := ivfreader.NewWith(file)
		if openErr != nil {
			panic(openErr)
		}

		// Determine video codec
		var trackCodec string
		switch header.FourCC {
		case "AV01":
			trackCodec = webrtc.MimeTypeAV1
		case "VP90":
			trackCodec = webrtc.MimeTypeVP9
		case "VP80":
			trackCodec = webrtc.MimeTypeVP8
		default:
			panic(fmt.Sprintf("Unable to handle FourCC %s", header.FourCC))
		}

		// Create a video track
		videoTrack, videoTrackErr := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: trackCodec}, "video", "pion")
		if videoTrackErr != nil {
			panic(videoTrackErr)
		}

		rtpSender, videoTrackErr := peerConnection.AddTrack(videoTrack)
		if videoTrackErr != nil {
			panic(videoTrackErr)
		}

		// Read incoming RTCP packets
		// Before these packets are returned they are processed by interceptors. For things
		// like NACK this needs to be called.
		go func() {
			rtcpBuf := make([]byte, 1500)
			for {
				if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
					return
				}
			}
		}()

		// Wait for connection established
		<-iceConnectedCtx.Done()

		go startVideoPlayback(videoTrack)
	}

	if haveAudioFile {
		// Create a audio track
		audioTrack, audioTrackErr := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, "audio", "pion")
		if audioTrackErr != nil {
			panic(audioTrackErr)
		}

		rtpSender, audioTrackErr := peerConnection.AddTrack(audioTrack)
		if audioTrackErr != nil {
			panic(audioTrackErr)
		}

		// Read incoming RTCP packets
		// Before these packets are returned they are processed by interceptors. For things
		// like NACK this needs to be called.
		go func() {
			rtcpBuf := make([]byte, 1500)
			for {
				if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
					return
				}
			}
		}()

		// Wait for connection established
		<-iceConnectedCtx.Done()

		go startAudioPlayback(audioTrack)
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

	// Wait for the offer to be pasted
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offerStr,
	}
	//todo: read the offer
	//signal.Decode(signal.MustReadStdin(), &offer)

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	candidatesChannel := make(chan string)
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

	// Set the remote SessionDescription
	if err = peerConnection.SetRemoteDescription(offer); err != nil {
		panic(err)
	}

	// Create answer
	answer, err := peerConnection.CreateAnswer(&webrtc.AnswerOptions{})
	if err != nil {
		panic(err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	if err = peerConnection.SetLocalDescription(answer); err != nil {
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
	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

	logger.Info("Gathering complete. Answer set as local description\n" + answer.SDP)

	//time.Sleep(time.Duration(30))

	return answerSD.Marshal()

	// Output the answer in base64 so we can paste it in browser
	//todo: render the answer
	//fmt.Println(signal.Encode(*peerConnection.LocalDescription()))

	// Block forever
	//select {}
}
