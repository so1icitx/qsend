package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/so1icitx/qsend/internal/protocol"
)

type NetworkMeasurement struct {
	FileR     io.Reader
	FileW     io.Writer
	BytesRead int
}

func main() {
	// sets up a tls config for quic connection
	tlsConf := tls.Config{InsecureSkipVerify: true, NextProtos: []string{"h3"}} // in production REMOVE InsecureSkipVerify and uncomment below
	/*
		certPool := x509.NewCertPool()
		certRaw, err := os.ReadFile("server.pem")
		if err != nil {
			log.Fatalln(err)
		}
		ok := certPool.AppendCertsFromPEM(certRaw)
		if !ok {
			log.Fatalln("Failed to parse the cert!")
		}
	*/

	// sets up a connection with the endpoints via quic
	quicConn, err := quic.DialAddr(context.Background(), "159.65.121.63:1447", &tlsConf, nil)
	if err != nil {
		log.Fatalln(err)
	}

	// opens a bidirectional quic stream
	stream, err := quicConn.OpenStream()
	if err != nil {
		log.Fatalln(err)
	}

	// sets up the header
	header := &protocol.FileServerHeader{}
	if len(os.Args) < 3 {
		log.Println("Not enough arguments\ngo run . (U/D) (FILENAME)")
		return
	}
	header.Action = []rune(os.Args[1])[0]
	header.FileName = filepath.Base(os.Args[2])
	header.FileNameLen = uint8(len(header.FileName))
	packetSize := 1 + 1 + header.FileNameLen + 32

	// puts it in a byte slice so we can send on the wire
	buffer := make([]byte, packetSize)
	buffer[0] = byte(header.Action)
	buffer[1] = header.FileNameLen
	copy(buffer[2:len(header.FileName)+2], header.FileName)

	// prepares the reader and writer for the network measurement service
	measurementTool := &NetworkMeasurement{
		FileR:     nil,
		FileW:     nil,
		BytesRead: 0,
	}
	// switch case statement to correlate between downloading and uploading
	switch header.Action {

	case 'U':
		// opens an existing file
		file, err := os.Open(os.Args[2])
		if err != nil {
			log.Fatalln(err)
		}
		defer file.Close()

		// hashes the file & appends the hash to the header before sending it to the server
		hash := sha256.New()
		if _, err := io.Copy(hash, file); err != nil {
			log.Fatalln(err)
		}
		sum := hash.Sum(nil)
		copy(buffer[len(header.FileName)+2:], sum)
		_, err = stream.Write(buffer)
		if err != nil {
			log.Fatalln(err)
		}

		// starts the background service that will calculate the network upload bandwidth
		measurementTool.FileR = file
		go measurementTool.HowFast()
		time.Sleep(time.Millisecond * 25)

		// resets the file offset and sends the header on the wire
		file.Seek(0, 0)
		_, err = io.Copy(stream, measurementTool)
		if err != nil {
			log.Fatalln(err)
		}
		stream.Close()

		// validates if the server got the correct file but no retransmission mechanism if data is corrupted
		receiptBuffer := make([]byte, 128)
		n, err := stream.Read(receiptBuffer)
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Printf("\nServer Response: %s\n", string(receiptBuffer[:n]))

	case 'D':
		// writes header without changes to the quic stream
		_, err = stream.Write(buffer)
		if err != nil {
			log.Fatalln(err)
		}

		// prepares the file
		f, err := os.OpenFile(header.FileName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			log.Fatalln(err)
		}
		defer f.Close()

		// starts the background service that will calculate the network download bandwidth
		measurementTool.FileW = f
		go measurementTool.HowFast()
		time.Sleep(time.Millisecond * 25)

		// writes to the file and indicating to the server that we received the file
		_, err = io.Copy(measurementTool, stream)
		if err != nil {
			log.Fatalln(err)
		}
		_, err = stream.Write([]byte("Received successfully"))
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func (m *NetworkMeasurement) Read(b []byte) (n int, err error) {
	n, err = m.FileR.Read(b)
	m.BytesRead += n
	return
}
func (m *NetworkMeasurement) Write(b []byte) (n int, err error) {
	n, err = m.FileW.Write(b)
	m.BytesRead += n
	return
}

func (m *NetworkMeasurement) HowFast() {
	for {
		time.Sleep(time.Second * 1)
		fmt.Printf("\r%.2f mbp/s         ", float32(m.BytesRead)/1048576.0)
		m.BytesRead = 0
	}
}
