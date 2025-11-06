package config

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// parseYAML parses the specific two-level mapping used by config.yaml
func parseYAML(r io.Reader, cfg *Config) error {
	type section int
	const (
		none section = iota
		db
		rm
		ws
		sv
		jw
	)

	scanner := bufio.NewScanner(r)
	var cur section

	lineNo := 0
	seenTop := map[section]bool{}

	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()

		// strip comments
		if i := strings.IndexByte(raw, '#'); i >= 0 {
			raw = raw[:i]
		}

		line := strings.TrimRight(raw, " \t\r\n")
		if strings.TrimSpace(line) == "" {
			continue
		}

		// top-level section? (no leading spaces)
		if len(line) > 0 && (line[0] != ' ' && line[0] != '\t') {
			switch strings.TrimSpace(line) {
			case "database:":
				cur = db
				if seenTop[db] {
					return fmt.Errorf("line %d: duplicate 'database' section", lineNo)
				}
				seenTop[db] = true
			case "rabbitmq:":
				cur = rm
				if seenTop[rm] {
					return fmt.Errorf("line %d: duplicate 'rabbitmq' section", lineNo)
				}
				seenTop[rm] = true
			case "websocket:":
				cur = ws
				if seenTop[ws] {
					return fmt.Errorf("line %d: duplicate 'websocket' section", lineNo)
				}
				seenTop[ws] = true
			case "services:":
				cur = sv
				if seenTop[sv] {
					return fmt.Errorf("line %d: duplicate 'services' section", lineNo)
				}
				seenTop[sv] = true
			case "jwt:":
				cur = jw
				if seenTop[jw] {
					return fmt.Errorf("line %d: duplicate 'jwt' section", lineNo)
				}
				seenTop[jw] = true
			default:
				return fmt.Errorf("line %d: unknown top-level key %q", lineNo, strings.TrimSuffix(strings.TrimSpace(line), ":"))
			}
			continue
		}

		// expect indented "key: value"
		if cur == none {
			return fmt.Errorf("line %d: key without a section", lineNo)
		}
		trim := strings.TrimSpace(line)
		colon := strings.IndexByte(trim, ':')
		if colon <= 0 {
			return fmt.Errorf("line %d: expected 'key: value'", lineNo)
		}
		key := strings.TrimSpace(trim[:colon])
		val := strings.TrimLeft(strings.TrimSpace(trim[colon+1:]), " \t")

		switch cur {
		case db:
			switch key {
			case "host":
				cfg.Database.Host = resolveScalar(val)
			case "port":
				p, err := strconv.Atoi(resolveScalar(val))
				if err != nil {
					return fmt.Errorf("line %d: database.port must be int: %v", lineNo, err)
				}
				cfg.Database.Port = p
			case "user":
				cfg.Database.User = resolveScalar(val)
			case "password":
				cfg.Database.Password = resolveScalar(val)
			case "database":
				cfg.Database.Name = resolveScalar(val)
			default:
				return fmt.Errorf("line %d: unknown key in database: %q", lineNo, key)
			}
		case rm:
			switch key {
			case "host":
				cfg.RabbitMQ.Host = resolveScalar(val)
			case "port":
				p, err := strconv.Atoi(resolveScalar(val))
				if err != nil {
					return fmt.Errorf("line %d: rabbitmq.port must be int: %v", lineNo, err)
				}
				cfg.RabbitMQ.Port = p
			case "user":
				cfg.RabbitMQ.User = resolveScalar(val)
			case "password":
				cfg.RabbitMQ.Password = resolveScalar(val)
			default:
				return fmt.Errorf("line %d: unknown key in rabbitmq: %q", lineNo, key)
			}
		case ws:
			switch key {
			case "port":
				p, err := strconv.Atoi(resolveScalar(val))
				if err != nil {
					return fmt.Errorf("line %d: websocket.port must be int: %v", lineNo, err)
				}
				cfg.WebSocket.Port = p
			default:
				return fmt.Errorf("line %d: unknown key in websocket: %q", lineNo, key)
			}
		case sv:
			switch key {
			case "ride_service":
				p, err := strconv.Atoi(resolveScalar(val))
				if err != nil {
					return fmt.Errorf("line %d: services.ride_service must be int: %v", lineNo, err)
				}
				cfg.Services.RideServicePort = p
			case "driver_location_service":
				p, err := strconv.Atoi(resolveScalar(val))
				if err != nil {
					return fmt.Errorf("line %d: services.driver_location_service must be int: %v", lineNo, err)
				}
				cfg.Services.DriverLocationServicePort = p
			case "admin_service":
				p, err := strconv.Atoi(resolveScalar(val))
				if err != nil {
					return fmt.Errorf("line %d: services.admin_service must be int: %v", lineNo, err)
				}
				cfg.Services.AdminServicePort = p
			default:
				return fmt.Errorf("line %d: unknown key in services: %q", lineNo, key)
			}
		case jw:
			switch key {
			case "secret_key":
				cfg.JWT.SecretKey = resolveScalar(val)
			default:
				return fmt.Errorf("line %d: unknown key in jwt: %q", lineNo, key)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

// resolveScalar trims whitespace and removes surrounding quotes from YAML-like scalars.
// For example:
//
//	"localhost"  -> localhost
//	'password123' -> password123
//	localhost     -> localhost
//
// This ensures values like jwt.secret_key are not stored with extra quotes.
func resolveScalar(s string) string {
	s = strings.TrimSpace(s)

	// if value is quoted with "..." or '...', remove quotes safely
	n := len(s)
	if n >= 2 {
		if (s[0] == '"' && s[n-1] == '"') || (s[0] == '\'' && s[n-1] == '\'') {
			if unq, err := strconv.Unquote(s); err == nil {
				return unq
			}
			// fallback if strconv.Unquote fails (e.g., mismatched quotes)
			return s[1 : n-1]
		}
	}

	return s
}
