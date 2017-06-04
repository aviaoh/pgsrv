package postgressrv

import (
    "fmt"
    "bytes"
    "encoding/binary"
)

// Version returns the protocol version supported by the client. The version is
// encoded by two consequtive 2-byte integers, one for the major version, and
// the other for the minor version. Currently version 3.0 is the only valid
// version.
func (m Msg) StartupVersion() (string, error) {
    if m.Type() != 0 {
        return "", Errf("Not an untyped startup message: %q", m.Type())
    }

    major := int(binary.BigEndian.Uint16(m[4:6]))
    minor := int(binary.BigEndian.Uint16(m[6:8]))
    return fmt.Sprintf("%d.%d", major, minor), nil
}

// StartupArgs parses the arguments delivered in the Startup and returns them
// as a key-value map. Startup messages contains a map of arguments, like the
// requested database name, user name, charset and additional connection
// defaults that may be used by the server. These arguments are encoded as pairs
// of key-values, terminated by a NULL character.
func (m Msg) StartupArgs() (map[string]string, error) {
    if m.Type() != 0 {
        return nil, Errf("Not an untyped startup message: %q", m.Type())
    }

    buff := m[8:] // skip the length (4-bytes) and version (4-bytes)

    // first create a single long list of strings, combining both keys and
    // values alternatingly
    var strings []string
    for len(buff) > 0 {

        // search for the next NULL terminator
        idx := bytes.IndexByte(buff, 0)
        if idx == -1 {
            break // none found, we're done.
        }

        // convert it to a string and append to the list
        strings = append(strings, string(buff[:idx]))

        // skip to the next terminator index for the next string
        buff = buff[idx + 1:]
    }

    // convert the list of strings to a map for key-value
    // all even indexes are keys, odd are values
    args := make(map[string]string)
    for i := 0 ; i < len(strings) - 1 ; i += 2 {
        args[strings[i]] = strings[i + 1]
    }

    return args, nil
}

// IsTLSRequest determines if this startup message is actually a request to open
// a TLS connection, in which case the version number is a special, predefined
// value of "1234.5679"
func (m Msg) IsTLSRequest() bool {
    v, _ := m.StartupVersion()
    return v == "1234.5679"
}

// IsInternal determines if this startup message is actually a peer node
// connecting for internal communication. We're keeping with Postgre's current
// approach of using a special version number. NOTE that internal connections
// do not use the standard postgres protocol.
func (m Msg) IsInternal() bool {
    if m.Type() != 0 {
        return false
    }

    return string(m[4:]) == "EP01"
}

// IsTerminate determines if the current message is a notification that the
// client has terminated the connection upon user-request.
func (m Msg) IsTerminate() bool {
    return m.Type() == 'X'
}

// NewTLSResponse creates a new single byte message indicating if the server
// supports TLS or not. If it does, the client must immediately proceed to
// initiate the TLS handshake
func TLSResponseMsg(supported bool) Msg {
    b := map[bool]byte{true: 'S', false: 'N'}[supported]
    return Msg([]byte{b})
}

// NewAuthOK creates a new message indicating that the authentication was
// successful
func AuthOKMsg() Msg {
    return []byte{'R', 0, 0, 0, 8, 0, 0, 0, 0}
}

// KeyDataMsg creates a new message providing the client with a process ID and
// secret key that it can later use to cancel running queries
func KeyDataMsg(pid int32, secret int32) Msg {
    msg := []byte{'K', 0, 0, 0, 12, 0, 0, 0, 0, 0, 0, 0, 0}
    binary.BigEndian.PutUint32(msg[5:9], uint32(pid))
    binary.BigEndian.PutUint32(msg[9:13], uint32(secret))
    return msg
}

func (m Msg) IsCancel() bool {
    v, _ := m.StartupVersion()
    return v == "1234.5678"
}

func (m Msg) CancelKeyData() (int32, int32, error) {
    if !m.IsCancel() {
        return -1, -1, fmt.Errorf("Not a cancel message")
    }

    pid := int32(binary.BigEndian.Uint32(m[8:12]))
    secret := int32(binary.BigEndian.Uint32(m[12:16]))
    return pid, secret, nil
}