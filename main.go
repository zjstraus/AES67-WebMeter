package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"gortc.io/sdp"
	"log"
	"math"
	"net"
	"regexp"
	"strconv"
	"time"
)

type AudioStats struct {
	RMS    float64
	Latest float64
	Peak   float64
}

type SAPMessage struct {
	Version       int
	Delete        bool
	Encrypted     bool
	Compressed    bool
	AuthLength    uint8
	MessageIDHash uint16
	Source        net.IP
	AuthData      []byte
	PayloadType   string
	Payload       []byte
}

func clen(n []byte) int {
	for i := 0; i < len(n); i++ {
		if n[i] == 0 {
			return i
		}
	}
	return len(n)
}

func ParseSAPMessage(data []byte) (*SAPMessage, error) {
	newMessage := SAPMessage{}
	// First two bytes are a header with some flags
	newMessage.Version = int(data[0] & 7)
	IPv6 := (data[0] & 8) > 0
	newMessage.Delete = (data[0] & 32) > 0
	newMessage.Encrypted = (data[0] & 64) > 0
	newMessage.Compressed = (data[0] & 128) > 0
	newMessage.AuthLength = data[1]

	// Rest of message has variable length and optional items
	var authOffset uint8
	var payloadTypeOffset uint8
	var payloadOffset uint8

	newMessage.MessageIDHash = binary.LittleEndian.Uint16(data[2:4])

	// Originating Source can be IPv4 or 6
	authOffset = 8
	if IPv6 {
		newMessage.Source = data[4:21]
		authOffset = 20
	} else {
		newMessage.Source = data[4:8]
	}

	// Optional Authentication Data
	if newMessage.AuthLength > 0 {
		newMessage.AuthData = data[authOffset : authOffset+newMessage.AuthLength]
	}

	// Optional Payload Type
	payloadTypeOffset = authOffset + newMessage.AuthLength
	payloadTypeLength := uint8(clen(data[payloadTypeOffset:]))
	if payloadTypeLength > 0 {
		newMessage.PayloadType = string(data[payloadTypeOffset : payloadTypeOffset+payloadTypeLength])
	}

	// Grab payload
	payloadOffset = payloadTypeOffset + payloadTypeLength
	newMessage.Payload = data[payloadOffset:]

	return &newMessage, nil
}

func main() {
	interfaceName := flag.String("interface", "", "Network interface to listen on")
	streamName := flag.String("stream", "", "AES67 stream name to receive")
	listInterfaces := flag.Bool("listinterfaces", false, "List potential network interfaces and quit")
	sapAddress := flag.String("sapaddress", "239.255.255.255:9875", "Address to monitor for SAP announcements")
	flag.Parse()

	if *listInterfaces {
		interfaces, interfaceErr := net.Interfaces()
		if interfaceErr != nil {
			log.Fatalf("Could not get interface list: %s", interfaceErr)
		}
		fmt.Println("Interfaces:")
		for _, iface := range interfaces {
			fmt.Printf("%s\n", iface.Name)
		}
		return
	}

	netinterface, interfaceErr := net.InterfaceByName(*interfaceName)
	if interfaceErr != nil {
		log.Fatalf("Could not open interface '%s': %s'", *interfaceName, interfaceErr)
	}

	log.Printf("Starting HTTP server")
	data := make(chan []AudioStats)
	go serveHTTP(data)

	// Set up to receive SAP messages
	address, _ := net.ResolveUDPAddr("udp4", *sapAddress)
	conn, saplistenerr := net.ListenMulticastUDP("udp4", netinterface, address)
	if saplistenerr != nil {
		log.Fatalf("Could not open SAP listener: %s", saplistenerr)
	}

	if *streamName == "" {
		log.Fatal("Stream name is required")
	}

	bufferErr := conn.SetReadBuffer(1500)
	if bufferErr != nil {
		log.Fatalf("Error setting UDP receive buffer %s", bufferErr)
	}
	buffer := make([]byte, 1500)
	var sdpSession sdp.Session
	for {
		numBytes, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Print("UDP read error", err)
		}
		//log.Printf("Received %d from %s", numBytes, src)
		sap, _ := ParseSAPMessage(buffer[:numBytes])
		//log.Printf("SAP source is %s", sap.Source)
		if sap.PayloadType == "application/sdp" {
			sdpSession, sessionerr := sdp.DecodeSession(sap.Payload, sdpSession)
			if sessionerr != nil {
				log.Fatalf("Session :%s", sessionerr)
			}

			sdpDecode := sdp.NewDecoder(sdpSession)
			m := new(sdp.Message)

			if err = sdpDecode.Decode(m); err != nil {
				log.Fatal("err:", err)
			}
			log.Println("Decoded SAP session", m.Name)
			if m.Name == *streamName {
				bindStream(m, 300, time.Second/60, netinterface, data)
			}
		}
	}
}

func FStoDBFS(x float64) float64 {
	realval := 8.6562 * math.Log(x)
	if realval < -100 {
		return -100
	}
	return realval
}

func bindStream(session *sdp.Message, window int, updateFreqency time.Duration, netinterface *net.Interface, dataChan chan []AudioStats) {
	// Parse out stream details
	channels := 8
	sampleRate := 48000
	for _, attribute := range session.Medias[0].Attributes {
		if attribute.Key == "rtpmap" {
			channelFinder := regexp.MustCompile(`L\d*/(\d*)/(\d*)`)
			channelString := channelFinder.FindAllStringSubmatch(attribute.Value, -1)
			if len(channelString) > 0 {
				sampleRate, _ = strconv.Atoi(channelString[0][1])
				channels, _ = strconv.Atoi(channelString[0][2])
				log.Printf("Discovered %d stream channels at %d Hz\n", channels, sampleRate)
			} else {
				log.Printf("Assuming default %d stream channels at %d Hz\n", channels, sampleRate)
			}
		}
	}

	// Parse out the stream address & connect
	streamAddress := net.UDPAddr{
		IP:   session.Connection.IP,
		Port: session.Medias[0].Description.Port,
	}
	log.Printf("Binding audio on interface %s\n", netinterface.Name)
	conn, err := net.ListenMulticastUDP("udp4", netinterface, &streamAddress)
	if err != nil {
		log.Printf("Error listenting for stream on %s", netinterface.Name)
		log.Fatal(err)
	}

	// Size read buffer for standard ethernet frames
	bufferErr := conn.SetReadBuffer(1500)
	if bufferErr != nil {
		log.Printf("Error setting UDP receive buffer %s", bufferErr)
	}
	buffer := make([]byte, 1500)
	log.Printf("Bound to audio stream on %s:%d\n", streamAddress.IP, streamAddress.Port)

	// Preallocate all the audio processing data since it runs for every sample on every channel
	windowBuffer := make([][96000]float64, channels)
	usedWindow := (sampleRate * window) / 1000

	channelData := make([][48]int, channels)
	var amplitudeFS float64
	var amplitudeFSSquared float64
	windowTotal := make([]float64, channels)
	var previousValue float64
	windowIndex := 0
	peaks := make([]float64, channels)
	latests := make([]float64, channels)

	// Prepare output sender
	outputTicker := time.NewTicker(updateFreqency)
	stats := make([]AudioStats, channels)
	go func() {
		for {
			select {
			case <-outputTicker.C:
				for channel := 0; channel < channels; channel++ {
					stats[channel].RMS = FStoDBFS(math.Sqrt(windowTotal[channel] / 12000))
					stats[channel].Latest = FStoDBFS(latests[channel])
					stats[channel].Peak = FStoDBFS(peaks[channel])
				}
				dataChan <- stats
			}
		}
	}()

	for {
		numBytes, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Print("UDP read error", err)
		}
		bytesRead := 12 // skip RTP header
		sample := 0
		for bytesRead < numBytes {
			for channel := 0; channel < channels; channel++ {
				// Assumes 24 bit streams
				level := int(buffer[bytesRead]&0x7f)<<16 + int(buffer[bytesRead+1])<<8 + int(buffer[bytesRead+2]) - int(buffer[bytesRead]&0x80)<<16
				channelData[channel][sample] = level
				bytesRead += 3
			}
			sample++
		}

		for i := 0; i < sample; i++ {
			for channel := 0; channel < channels; channel++ {
				amplitudeFS = math.Abs(float64(channelData[channel][i]) / (1 << 23))
				amplitudeFSSquared = math.Pow(amplitudeFS, 2)
				peaks[channel] = math.Max(amplitudeFS, peaks[channel])
				latests[channel] = amplitudeFS
				previousValue = windowBuffer[channel][windowIndex]
				windowBuffer[channel][windowIndex] = amplitudeFSSquared
				windowTotal[channel] += amplitudeFSSquared
				windowTotal[channel] -= previousValue
			}
			if windowIndex == usedWindow {
				windowIndex = 0
			} else {
				windowIndex++
			}
		}
	}
}
