package protocol

// FileServerHeader defines the binary envelope for the qsend protocol.
type FileServerHeader struct {
	Action      rune
	FileNameLen uint8
	FileName    string
	FileHash    [32]byte
}
