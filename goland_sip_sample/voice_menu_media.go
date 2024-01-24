package main

import "C"
import (
	"context"
	"errors"
	"fmt"
	"github.com/pion/interceptor"
	"github.com/pion/logging"
	"github.com/pion/sdp"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/oggreader"
	"github.com/golang/freetype/truetype"
	"image"
	"image/color"
	"image/draw"
	"io"
	"io/ioutil"
	"log"
	"os"
	"time"
)

const (
	greetingAudioFileName   = "./resources/greeting.ogg"
	dtmfAudioFileName   	= "./resources/dtmf.ogg"
	fontFile			   	= "/usr/share/fonts/truetype/JetBrainsMono/JetBrainsMono-Regular.ttf"
	audioOggPageDuration = time.Millisecond * 20
)

var (
	RGBA_COLOR_RED = color.RGBA{200, 0, 0, 0xFF}
	RGBA_COLOR_BLACK = color.RGBA{0, 0, 0, 0xFF}
	RGBA_COLOR_WHITE = color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
	RGBA_COLOR_GRAD_LIGHT = color.RGBA{0xE8, 0xE6, 0xE2, 0xFF}
	RGBA_COLOR_GRAD_BLUE = color.RGBA{0xDC, 0xE5, 0xF7, 0xFF}
	RGBA_COLOR_ORANGE    = color.RGBA{0xff, 0x64, 0x27, 0xFF}
)

type OggAudioPage struct {
	pageData 	[]byte
	pageHeader	*oggreader.OggPageHeader
}

type VoiceMenuResources struct {
	greetingAudioPages	[]OggAudioPage
	dtmfAudioPages		[]OggAudioPage
	defaultFont			*truetype.Font
}

func readOggFile(path string) []OggAudioPage {
	var result []OggAudioPage
	file, oggErr := os.Open(path)
	if oggErr != nil {
		panic(oggErr)
	}

	// Open on oggfile in non-checksum mode.
	ogg, _, oggErr := oggreader.NewWith(file)
	if oggErr != nil {
		panic(oggErr)
	}

	for ; true; {
		pageData, pageHeader, oggErr := ogg.ParseNextPage()
		if errors.Is(oggErr, io.EOF) {
			break
			//os.Exit(0)
		}

		result = append(result, OggAudioPage{pageData, pageHeader})
	}
	return result
}

func (vrm *VoiceMenuResources) init() {
	vrm.dtmfAudioPages 		= readOggFile(dtmfAudioFileName)
	vrm.greetingAudioPages	= readOggFile(greetingAudioFileName)

	fontBytes, err := ioutil.ReadFile(fontFile)
	if err != nil {
		log.Println(err)
		return
	}
	f, err := truetype.Parse(fontBytes)
	if err != nil {
		log.Println(err)
		return
	}

	vrm.defaultFont = f
}


type VoiceMenuInstance struct {
	_peerConnection 		*webrtc.PeerConnection
	_iceConnectedCtx		context.Context
	_iceConnectedCtxCancel 	context.CancelFunc
	_videoTrack				*webrtc.TrackLocalStaticSample
	_videoTrackSender		*webrtc.RTPSender
	_videoTrackFPS			int
	_audioTrack				*webrtc.TrackLocalStaticSample
	_audioTrackSender		*webrtc.RTPSender
	_encoder				*Encoder
	_vmr					*VoiceMenuResources
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
			//time.Sleep(time.Second * time.Duration(2))
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
	e, err := NewEncoder(CODEC_ID_H264, image.NewRGBA(image.Rect(0,0,1280,720)), vmi._videoTrackFPS)
	if err != nil {panic(err)}
	vmi._encoder = e
}

func initMediaTrack(
	peerConnection *webrtc.PeerConnection,
	codecCapability webrtc.RTPCodecCapability,
	id string,
	streamId string )(*webrtc.TrackLocalStaticSample, *webrtc.RTPSender) {
	track, trackErr := webrtc.NewTrackLocalStaticSample(
		codecCapability,
		id,
		streamId,
	)
	if trackErr != nil {
		panic(trackErr)
	}

	rtpSender, trackErr := peerConnection.AddTrack(track)
	if trackErr != nil {
		panic(trackErr)
	}

	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	return track, rtpSender
}

func initVideoTrack(peerConnection *webrtc.PeerConnection) (*webrtc.TrackLocalStaticSample, *webrtc.RTPSender){
	return initMediaTrack(
		peerConnection,
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 60},
		"video",
		"pion",
		)
}

func initAudioTrack(peerConnection *webrtc.PeerConnection) (*webrtc.TrackLocalStaticSample, *webrtc.RTPSender){
	return initMediaTrack(
		peerConnection,
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		"audio",
		"pion",
	)
}

func NewVoiceMenuInstance(vmr *VoiceMenuResources, videoFPS int ) *VoiceMenuInstance {
	var vmi = &VoiceMenuInstance{}
	vmi._vmr = vmr
	vmi._videoTrackFPS = videoFPS
	return vmi
}

func (vmi *VoiceMenuInstance) connect(offerStr string, audio bool, video bool) string {
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
	if video {
		vmi._videoTrack, vmi._videoTrackSender = initVideoTrack(vmi._peerConnection)
	}
	if audio {
		vmi._audioTrack, vmi._audioTrackSender = initAudioTrack(vmi._peerConnection)
	}

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

func playbackTrack(vmi *VoiceMenuInstance, track []OggAudioPage ) {
	ticker := time.NewTicker(audioOggPageDuration)
	var lastGranule uint64
	totalPages := len(track)

	logger.Info("Start track playback. Num samples: ", len(track))

	for frameIdx := 0 ; frameIdx < totalPages; frameIdx++ {
		<-ticker.C

		page := track[frameIdx]

		// The amount of samples is the difference between the last and current timestamp
		sampleCount := float64(page.pageHeader.GranulePosition - lastGranule)
		lastGranule = page.pageHeader.GranulePosition
		sampleDuration := time.Duration(sampleCount/48) * time.Millisecond
		logger.Info("Sample duration ", sampleDuration, " Granule Position: ", page.pageHeader.GranulePosition)

		if err := vmi._audioTrack.WriteSample(media.Sample{Data: page.pageData, Duration: sampleDuration}); err != nil {
			panic(err)
		}
	}
}

func (vmi *VoiceMenuInstance) StartAudioPlayback() {
	<-vmi._iceConnectedCtx.Done()

	time.Sleep(time.Second)

	// Keep track of last granule, the difference is the amount of samples in the buffer


	// It is important to use a time.Ticker instead of time.Sleep because
	// * avoids accumulating skew, just calling time.Sleep didn't compensate for the time spent parsing the data
	// * works around latency issues with Sleep (see https://github.com/golang/go/issues/44343)

	//time.Sleep(time.Duration(10) * time.Second)

	playbackTrack(vmi, vmi._vmr.greetingAudioPages)
	//playbackTrack(vmi, vmi._vmr.dtmfAudioPages)
	for ;true; {
		time.Sleep(time.Second*5)
		playbackTrack(vmi, vmi._vmr.dtmfAudioPages)
	}

}

func (vmi *VoiceMenuInstance) StartPlayback() {
	if vmi._audioTrack != nil {
		go vmi.StartAudioPlayback()
	}
	if vmi._videoTrack != nil {
		go vmi.StartVideoPlayback()
	}
}

func (vmi *VoiceMenuInstance) StartVideoPlayback() {
	<-vmi._iceConnectedCtx.Done()

	numerator := 1
	denominator := vmi._videoTrackFPS
	videoDurationBetweenFrames := (float32(numerator)/float32(denominator))*1000
	logger.Info("Peer connection established. sending video. Between frames: ",
		videoDurationBetweenFrames,
		" milliseconds",
	)

	avPacket, freePacket := vmi._encoder.allocPacket()
	defer freePacket()

	ticker := time.NewTicker(time.Millisecond * time.Duration(videoDurationBetweenFrames))
	for i:= 0; true; i++  {
		inputImage := vmi._encoder.inputImage

		xShift := 2*i % (inputImage.Bounds().Dx() - 200) + 100

		draw.Draw(inputImage, inputImage.Bounds(), &image.Uniform{RGBA_COLOR_GRAD_LIGHT}, image.Point{}, draw.Src)
		draw.Draw(inputImage, image.Rect(xShift, 110, 100+xShift, 150), &image.Uniform{RGBA_COLOR_ORANGE}, image.Point{}, draw.Src)
		addLabel(vmi._vmr, inputImage, xShift, 100, "heyhey!!! DTMF coming soon!!!", RGBA_COLOR_ORANGE)
		addLabel(vmi._vmr, inputImage, 200, 200, fmt.Sprintf("Frame number %d", i), RGBA_COLOR_ORANGE)
		addLabel(vmi._vmr, inputImage, 200, 300, "public void JetBrainsMonoSpace(int here) { print(\"Hello World!\"); }", RGBA_COLOR_BLACK)

		//sometimes ffmpeg skips frames

		vmi._encoder.initPacket(avPacket, i)

		err, outSize := vmi._encoder.WriteFrame(avPacket)
		if err != nil {panic(err)}
		//keep feeding frames to ffmpeg, but don't display blanks
		if outSize < 0 {
			continue
		}

		<-ticker.C

		packetSlice := avPacketToSlice(avPacket)
		mediaSample := media.Sample{
			Data: packetSlice,
			Duration: time.Duration(float64(time.Millisecond) * float64(videoDurationBetweenFrames)),
			//Timestamp: time.Now(),
			PacketTimestamp: uint32(i),
		}
		if ivfErr := vmi._videoTrack.WriteSample(mediaSample) ; ivfErr != nil {
			panic(ivfErr)
		}
	}
}