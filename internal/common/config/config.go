package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DB  DB
	RMQ RMQ
	WS  WS
}

type DB struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
}

type RMQ struct {
	Host     string
	Port     int
	User     string
	Password string
}

type WS struct {
	Port int
}

func Load(cfgPath string) (*Config, error) {
	f, err := os.Open(cfgPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	
	var (
		lineNo int
		section string
		cfg Config
		seenDB = make(map[string]bool)
		seenRMQ = make(map[string]bool)
		seenWS = make(map[string]bool)
		requiredDB = []string{"host", "port", "user", "password", "database"}
		requiredRMQ = []string{"host", "port", "user", "password"}
		requiredWS = []string{"port"}
	)

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasSuffix(line, ":") && !strings.Contains(line, " ") && !strings.Contains("\t", line) {
			sec := strings.TrimSuffix(line, ":") 
			switch sec {
			case "database", "rabbitmq", "websocket":
				section = sec
			default:
				return nil, fmt.Errorf("line %d: unknown section %s", lineNo, sec)
			}
			continue
		}

		if section == "" {
			return nil, fmt.Errorf("line %d: emtpy section <%s>", lineNo, section)
		}

		k, v, ok := splitKV(line)
		if !ok {
			return nil, fmt.Errorf("line %d: expected 'key: value'", lineNo)
		}

		v = trimQuotes(v)
		switch section {
		case "database":
			if seenDB[k] {
				return nil, fmt.Errorf("line %d, duplicate key %q in [database]", lineNo, k)
			}
			seenDB[k] = true
			switch k {
			case "port":
				p, err := strconv.Atoi(v)
				if err != nil {
					return nil, fmt.Errorf("line %d: database.port must be int: %w", lineNo, err)
				}
				cfg.DB.Port = p
			case "host":
				cfg.DB.Host = v
			case "user":
				cfg.DB.User = v
			case "password":
				cfg.DB.Password = v
			case "database":
				cfg.DB.Name = v
			default:
				return nil, fmt.Errorf("line %d: unknown field for [database]: %q", lineNo, v)
			}

		case "rabbitmq":
			if seenRMQ[k] {
				return nil, fmt.Errorf("line %d: duplicate key %q in [rabbitmq]", lineNo, k)
			}
			seenRMQ[k] = true
			switch k {
			case "host":
				cfg.RMQ.Host = v
			case "port":
				p, err := strconv.Atoi(v)
				if err != nil {
					return nil, fmt.Errorf("line %d: rabbitmq.port must be int: %v", lineNo, v)
				}
				cfg.RMQ.Port = p
			case "user":
				cfg.RMQ.User = v
			case "password":
				cfg.RMQ.Password = v
			default:
				return nil, fmt.Errorf("line %d: unkown field for [rabbitmq]: %q", lineNo, k)
			}
		case "websocket":
			if seenWS[k] {
				return nil, fmt.Errorf("line %d: duplicate key %q in [websocket]", lineNo, k)
			}
			seenWS[k] = true
			switch k {
			case "port":
				p, err := strconv.Atoi(v)
				if err != nil {
					return nil, err
				}
				cfg.RMQ.Port = p
			default:
				return nil, fmt.Errorf("line %d: unkown field for [websocket]: %q", lineNo, k)
			}
		}
	}

	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan config: %w", err)
	}

	if err := ensureRequired(seenDB, requiredDB, "database"); err != nil {
		return nil, err
	}

	if err := ensureRequired(seenRMQ, requiredRMQ, "rabbitmq"); err != nil {
		return nil, err
	}

	if err := ensureRequired(seenWS, requiredWS, "websocket"); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func splitKV(line string) (key, val string, ok bool) {
	i := strings.IndexRune(line, ':')
	if i <= 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:i])
	val = strings.TrimSpace(line[i+1:])
	if key == "" || val == "" {
		return "", "", false
	}
	return key, val, true
}

func trimQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') ||
			(s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func ensureRequired(seen map[string]bool, required []string, section string) error {
	var missing []string
	for _, k := range required {
		if !seen[k] {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return errors.New("missing required keys in [" + section + "]: " + strings.Join(missing, ", "))
	}
	return nil
}