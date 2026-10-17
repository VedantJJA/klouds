package minecraft

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	packetTypeAuthResp    int32 = 2
	packetTypeExecCommand int32 = 2
	packetTypeAuth        int32 = 3
	packetTypeResponse    int32 = 0
)

// SendRconCommand connects to an RCON server, authenticates, executes a command, and returns the response.
func SendRconCommand(addr, password, command string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return "", fmt.Errorf("dial rcon at %s: %w", addr, err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	// 1. Authenticate (packet type 3)
	authReqID := int32(42)
	if err := writePacket(conn, authReqID, packetTypeAuth, password); err != nil {
		return "", fmt.Errorf("write auth packet: %w", err)
	}

	respID, _, _, err := readPacket(conn)
	if err != nil {
		return "", fmt.Errorf("read auth response: %w", err)
	}
	if respID == -1 {
		return "", fmt.Errorf("rcon authentication rejected: invalid password")
	}

	// 2. Execute command (packet type 2)
	command = strings.TrimPrefix(command, "/")
	cmdReqID := int32(43)
	if err := writePacket(conn, cmdReqID, packetTypeExecCommand, command); err != nil {
		return "", fmt.Errorf("write command packet: %w", err)
	}

	_, _, body, err := readPacket(conn)
	if err != nil {
		return "", fmt.Errorf("read command response: %w", err)
	}

	return strings.TrimSpace(body), nil
}

func writePacket(w io.Writer, id, pType int32, payload string) error {
	payloadBytes := []byte(payload)
	// Packet size = 4 (ID) + 4 (Type) + len(body) + 2 (two null bytes)
	packetSize := int32(4 + 4 + len(payloadBytes) + 2)

	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, packetSize); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, id); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, pType); err != nil {
		return err
	}
	if _, err := buf.Write(payloadBytes); err != nil {
		return err
	}
	if _, err := buf.Write([]byte{0x00, 0x00}); err != nil {
		return err
	}

	_, err := w.Write(buf.Bytes())
	return err
}

func readPacket(r io.Reader) (id, pType int32, body string, err error) {
	var size int32
	if err = binary.Read(r, binary.LittleEndian, &size); err != nil {
		return 0, 0, "", err
	}
	if size < 10 || size > 65535 {
		return 0, 0, "", fmt.Errorf("invalid packet size: %d", size)
	}

	payload := make([]byte, size)
	if _, err = io.ReadFull(r, payload); err != nil {
		return 0, 0, "", err
	}

	id = int32(binary.LittleEndian.Uint32(payload[0:4]))
	pType = int32(binary.LittleEndian.Uint32(payload[4:8]))
	bodyBytes := payload[8:]
	if len(bodyBytes) >= 2 {
		bodyBytes = bodyBytes[:len(bodyBytes)-2]
	}
	body = string(bodyBytes)
	return id, pType, body, nil
}

// PlayerListInfo contains parsed Minecraft player info from a "list" command.
type PlayerListInfo struct {
	OnlinePlayers []string `json:"online_players"`
	Current       int      `json:"current"`
	Max           int      `json:"max"`
}

var listRegex = regexp.MustCompile(`(?i)There are (\d+) (?:of a max (?:of )?)?(\d+) players online(?::\s*(.*))?`)

// ParsePlayerList parses standard Minecraft list outputs such as:
// "There are 2 of a max of 20 players online: Steve, Alex"
// or "There are 0 of a max 20 players online:"
func ParsePlayerList(output string) PlayerListInfo {
	info := PlayerListInfo{
		OnlinePlayers: []string{},
	}

	clean := strings.TrimSpace(output)
	matches := listRegex.FindStringSubmatch(clean)
	if len(matches) >= 3 {
		if c, err := strconv.Atoi(matches[1]); err == nil {
			info.Current = c
		}
		if m, err := strconv.Atoi(matches[2]); err == nil {
			info.Max = m
		}
		if len(matches) >= 4 && strings.TrimSpace(matches[3]) != "" {
			names := strings.Split(matches[3], ",")
			for _, n := range names {
				t := strings.TrimSpace(n)
				if t != "" {
					info.OnlinePlayers = append(info.OnlinePlayers, t)
				}
			}
		}
	}

	return info
}
