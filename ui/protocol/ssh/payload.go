package sshui

// Payload is the SSH connect form state stored in protocol.Draft.Payload.
type Payload struct {
	Host        string
	Port        int
	User        string
	Password    string
	KeyPath     string
	KeyPassword string
	CertPath    string
}

// DefaultPayload returns a blank SSH form with port 22.
func DefaultPayload() Payload {
	return Payload{Port: 22}
}
