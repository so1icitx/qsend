package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/so1icitx/qsend/internal/protocol"
)

func main() {
	// generates a ECDSA private key that will be used for signing
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatalln(err)
	}

	// creates a very basic x509 cert blueprint for testing
	certTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore:             time.Now().Truncate(time.Hour * 1),
		NotAfter:              time.Now().Add(time.Hour * 10),
		DNSNames:              []string{"localhost"},
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// creates the actual certificate
	cert, err := x509.CreateCertificate(rand.Reader, certTemplate, certTemplate, &privateKey.PublicKey, privateKey)
	if err != nil {
		log.Fatalln(err)
	}

	/*  Uncomment below code if you want your client to be able to verify the server WHICH IS NOT A QUESTION IF IN PRODUCTION
	f, err := os.OpenFile("../client/server.pem", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalln(err)
	}
	defer f.Close()
	err = pem.Encode(f, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert,
	})
	if err != nil {
		log.Fatalln(err)
	}
	*/

	// tls configuration semantics
	tlsCert := tls.Certificate{
		Certificate: [][]byte{cert},
		PrivateKey:  privateKey,
	}
	tlsConf := tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"h3"},
	}

	// binds a listener and awaits for quic traffic
	listener, err := quic.ListenAddr("localhost:1447", &tlsConf, nil)
	if err != nil {
		log.Fatalln(err)
	}

	for {
		// accepts a quic connection and asynchronously handles the client
		conn, err := listener.Accept(context.Background())
		if err != nil {
			log.Println(err)
			continue
		}
		go HandleClient(conn)
	}
}
func HandleClient(conn *quic.Conn) {
	// creates a stream from the quic connection
	stream, err := conn.AcceptStream(context.Background())
	if err != nil {
		log.Println(err)
		return
	}

	// reads the client header and stores it in a buffer
	data := make([]byte, 289)
	_, err = stream.Read(data)
	if err != nil {
		log.Println(err)
		return
	}

	// populates the clientHeader struct
	fileNameoffset := data[1]
	fileNamend := data[2 : fileNameoffset+2]
	clientHeader := &protocol.FileServerHeader{
		Action:      rune(data[0]),
		FileNameLen: fileNameoffset,
		FileName:    string(fileNamend),
	}
	copy(clientHeader.FileHash[:], data[fileNameoffset+2:])

	// switch case based on whether the client wants to download or upload a file
	switch clientHeader.Action {
	case 'U':

		// prepares to hash the received file and compare hashes
		hash := sha256.New()
		file, err := os.OpenFile(clientHeader.FileName, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600)
		if err != nil {
			log.Println(err)
			stream.Close()
			return
		}
		defer file.Close()

		// populates the file & resets the file offset to the start
		io.Copy(file, stream)
		file.Seek(0, 0)

		// hashes the file
		if _, err := io.Copy(hash, file); err != nil {
			log.Println(err)
			stream.Close()
			return
		}
		sum := hash.Sum(nil)

		// validates that the hashes match
		if string(sum) != string(clientHeader.FileHash[:]) {
			stream.Write([]byte("Hash mismatch"))
		} else {
			stream.Write([]byte("OK"))
		}

	case 'D':
		file, err := os.Open(clientHeader.FileName)
		if err != nil {
			log.Println(err)
			stream.Close()
			return
		}
		defer file.Close()

		// copies the file to the stream efficiently
		io.Copy(stream, file)
		stream.Close()

		// awaits approval from the client that they received the file successfully
		receiptBuffer := make([]byte, 128)
		_, err = stream.Read(receiptBuffer)
		if err != nil {
			log.Println(err)
			return
		}
	}
}
