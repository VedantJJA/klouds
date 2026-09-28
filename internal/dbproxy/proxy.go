package dbproxy

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// DatabaseProxy provides a single-port external TCP routing gateway for PostgreSQL and Redis databases.
// External clients (psql, DBeaver, Prisma, TablePlus, redis-cli) connect to a single public port,
// and the router inspects the connection protocol (StartupMessage / SNI / Auth) to dynamically
// splice traffic to the isolated target container on the internal Docker network.
type DatabaseProxy struct {
	pool      *pgxpool.Pool
	domain    string
	pgPort    int
	redisPort int
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// NewDatabaseProxy creates a new single-port database proxy gateway.
func NewDatabaseProxy(pool *pgxpool.Pool, domain string, pgPort, redisPort int) *DatabaseProxy {
	return &DatabaseProxy{
		pool:      pool,
		domain:    domain,
		pgPort:    pgPort,
		redisPort: redisPort,
		stopCh:    make(chan struct{}),
	}
}

// Start begins listening on the configured external database ports.
func (p *DatabaseProxy) Start() error {
	if p.pgPort > 0 {
		pgListener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", p.pgPort))
		if err != nil {
			log.Warn().Err(err).Int("port", p.pgPort).Msg("Database proxy: could not bind PostgreSQL external port")
		} else {
			p.wg.Add(1)
			go p.servePostgres(pgListener)
			log.Info().Int("port", p.pgPort).Msg("Database proxy: PostgreSQL external router active")
		}
	}

	if p.redisPort > 0 {
		redisListener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", p.redisPort))
		if err != nil {
			log.Warn().Err(err).Int("port", p.redisPort).Msg("Database proxy: could not bind Redis external port")
		} else {
			p.wg.Add(1)
			go p.serveRedis(redisListener)
			log.Info().Int("port", p.redisPort).Msg("Database proxy: Redis external router active")
		}
	}

	return nil
}

// Stop shuts down the proxy listeners gracefully.
func (p *DatabaseProxy) Stop() {
	close(p.stopCh)
	p.wg.Wait()
}

func (p *DatabaseProxy) servePostgres(l net.Listener) {
	defer p.wg.Done()
	defer l.Close()

	go func() {
		<-p.stopCh
		_ = l.Close()
	}()

	for {
		clientConn, err := l.Accept()
		if err != nil {
			select {
			case <-p.stopCh:
				return
			default:
				time.Sleep(50 * time.Millisecond)
				continue
			}
		}

		go p.handlePostgresClient(clientConn)
	}
}

func (p *DatabaseProxy) handlePostgresClient(client net.Conn) {
	defer client.Close()
	_ = client.SetDeadline(time.Now().Add(15 * time.Second))

	// Read initial 8-byte header
	header := make([]byte, 8)
	if _, err := io.ReadFull(client, header); err != nil {
		return
	}

	pktLen := int(binary.BigEndian.Uint32(header[:4]))
	code := binary.BigEndian.Uint32(header[4:8])

	var startupBytes []byte

	// Check if client requested SSL (SSLRequest code: 80877103 = 0x04d2162f)
	if pktLen == 8 && code == 80877103 {
		// Respond 'N' to decline direct SSL negotiation and instruct client to proceed unencrypted
		// (Authentication credentials remain salted & hashed by SCRAM/MD5 inside the protocol)
		if _, err := client.Write([]byte{'N'}); err != nil {
			return
		}

		// Now read the real StartupMessage
		startupHeader := make([]byte, 8)
		if _, err := io.ReadFull(client, startupHeader); err != nil {
			return
		}
		startupLen := int(binary.BigEndian.Uint32(startupHeader[:4]))
		if startupLen < 8 || startupLen > 10000 {
			return
		}

		rest := make([]byte, startupLen-8)
		if _, err := io.ReadFull(client, rest); err != nil {
			return
		}

		startupBytes = append(startupHeader, rest...)
	} else if pktLen >= 8 && pktLen <= 10000 {
		// Direct StartupMessage without SSLRequest
		rest := make([]byte, pktLen-8)
		if _, err := io.ReadFull(client, rest); err != nil {
			return
		}
		startupBytes = append(header, rest...)
	} else {
		return
	}

	// Parse parameters from StartupMessage: bytes 8..end are null-terminated key/val pairs
	params := parsePGStartupParams(startupBytes)
	dbName := params["database"]
	userName := params["user"]

	if dbName == "" && userName == "" {
		sendPGError(client, "No database or user specified in connection request")
		return
	}

	// Resolve target container in database
	targetHost, targetPort, err := p.resolvePostgresTarget(dbName, userName)
	if err != nil {
		log.Warn().Str("database", dbName).Str("user", userName).Err(err).Msg("Database proxy: target not found")
		sendPGError(client, fmt.Sprintf("FATAL: database \"%s\" does not exist on this cluster\n", dbName))
		return
	}

	// Connect to internal database container
	backend, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", targetHost, targetPort), 5*time.Second)
	if err != nil {
		log.Error().Err(err).Str("host", targetHost).Int("port", targetPort).Msg("Database proxy: failed to dial backend container")
		sendPGError(client, "FATAL: database container is temporarily unreachable\n")
		return
	}
	defer backend.Close()

	// Clear deadlines for active session proxying
	_ = client.SetDeadline(time.Time{})
	_ = backend.SetDeadline(time.Time{})

	// Forward buffered StartupMessage to container
	if _, err := backend.Write(startupBytes); err != nil {
		return
	}

	// Splice traffic bidirectionally
	pipe(client, backend)
}

func (p *DatabaseProxy) resolvePostgresTarget(dbName, userName string) (string, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var internalHost string
	var internalPort int32

	query := `
		SELECT internal_host, internal_port
		FROM databases
		WHERE engine = 'postgresql'
		  AND (database_name = $1 OR username = $2 OR internal_host = $1 OR slug = $1)
		LIMIT 1`

	err := p.pool.QueryRow(ctx, query, dbName, userName).Scan(&internalHost, &internalPort)
	if err != nil {
		return "", 0, err
	}

	return internalHost, int(internalPort), nil
}

func (p *DatabaseProxy) serveRedis(l net.Listener) {
	defer p.wg.Done()
	defer l.Close()

	go func() {
		<-p.stopCh
		_ = l.Close()
	}()

	for {
		clientConn, err := l.Accept()
		if err != nil {
			select {
			case <-p.stopCh:
				return
			default:
				time.Sleep(50 * time.Millisecond)
				continue
			}
		}

		go p.handleRedisClient(clientConn)
	}
}

func (p *DatabaseProxy) handleRedisClient(client net.Conn) {
	defer client.Close()
	_ = client.SetDeadline(time.Now().Add(10 * time.Second))

	// Read initial greeting / AUTH command
	buf := make([]byte, 1024)
	n, err := client.Read(buf)
	if err != nil || n == 0 {
		return
	}

	initialBytes := buf[:n]

	// Extract password token from Redis command
	password := extractRedisPassword(initialBytes)

	// Resolve redis database
	targetHost, targetPort, err := p.resolveRedisTarget(password)
	if err != nil {
		_, _ = client.Write([]byte("-ERR authentication or database routing failed\r\n"))
		return
	}

	backend, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", targetHost, targetPort), 5*time.Second)
	if err != nil {
		_, _ = client.Write([]byte("-ERR backend unavailable\r\n"))
		return
	}
	defer backend.Close()

	_ = client.SetDeadline(time.Time{})
	_ = backend.SetDeadline(time.Time{})

	if _, err := backend.Write(initialBytes); err != nil {
		return
	}

	pipe(client, backend)
}

func (p *DatabaseProxy) resolveRedisTarget(password string) (string, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := p.pool.Query(ctx, `SELECT internal_host, internal_port, password_encrypted FROM databases WHERE engine = 'redis'`)
	if err != nil {
		return "", 0, err
	}
	defer rows.Close()

	var firstHost string
	var firstPort int32

	for rows.Next() {
		var host string
		var port int32
		var encPass string
		if err := rows.Scan(&host, &port, &encPass); err == nil {
			if firstHost == "" {
				firstHost = host
				firstPort = port
			}
			// If password matches or was passed
			if password != "" && strings.Contains(encPass, password) {
				return host, int(port), nil
			}
		}
	}

	// Fallback to first active redis database if single instance
	if firstHost != "" {
		return firstHost, int(firstPort), nil
	}

	return "", 0, fmt.Errorf("no redis target found")
}

func parsePGStartupParams(pkt []byte) map[string]string {
	params := make(map[string]string)
	if len(pkt) < 8 {
		return params
	}

	// Parameters start at byte 8
	data := pkt[8:]
	parts := bytes.Split(data, []byte{0})
	for i := 0; i+1 < len(parts); i += 2 {
		k := string(parts[i])
		v := string(parts[i+1])
		if k != "" {
			params[k] = v
		}
	}
	return params
}

func sendPGError(c net.Conn, msg string) {
	// PostgreSQL ErrorResponse packet: 'E' followed by int32 len, then 'M' field with message, then null byte
	errMsg := fmt.Sprintf("SFATAL\x00C28000\x00M%s\x00\x00", msg)
	pktLen := int32(len(errMsg) + 4)

	buf := new(bytes.Buffer)
	buf.WriteByte('E')
	_ = binary.Write(buf, binary.BigEndian, pktLen)
	buf.WriteString(errMsg)
	_, _ = c.Write(buf.Bytes())
}

func extractRedisPassword(data []byte) string {
	s := string(data)
	lines := strings.Split(s, "\r\n")
	for i, line := range lines {
		upper := strings.ToUpper(strings.TrimSpace(line))
		if upper == "AUTH" && i+1 < len(lines) {
			// Next non-length line
			for j := i + 1; j < len(lines); j++ {
				cand := strings.TrimSpace(lines[j])
				if !strings.HasPrefix(cand, "$") && cand != "" && !strings.EqualFold(cand, "default") {
					return cand
				}
			}
		}
		if strings.HasPrefix(upper, "AUTH ") {
			parts := strings.Fields(line)
			if len(parts) == 2 {
				return parts[1]
			} else if len(parts) == 3 {
				return parts[2]
			}
		}
	}
	return ""
}

func pipe(c1, c2 net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(c1, c2)
		if tcp, ok := c1.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
	}()

	go func() {
		defer wg.Done()
		_, _ = io.Copy(c2, c1)
		if tcp, ok := c2.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
	}()

	wg.Wait()
}
