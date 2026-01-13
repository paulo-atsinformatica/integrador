package trace

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

type EventType string

const (
	EventInsert EventType = "INSERT"
	EventUpdate EventType = "UPDATE"
	EventDelete EventType = "DELETE"
)

type TraceEvent struct {
	Type    EventType
	Table   string
	SQL     string
	TransID string
}

type Parser struct {
	mu           sync.RWMutex
	connections  map[string]string // ConnID -> DBPath
	transactions map[string]string // TransID -> ConnID

	// Buffer de eventos por transação (antes do commit)
	pendingEvents map[string][]*TraceEvent // TransID -> []Events

	// Estado atual por conexão
	lastEventPerConn map[string]string      // ConnID -> Last Event Name
	lastOPPerConn    map[string]*TraceEvent // ConnID -> Evento sendo montado

	// Regexes
	reHeader      *regexp.Regexp
	reAttach      *regexp.Regexp
	reTransaction *regexp.Regexp
	reStatement   *regexp.Regexp
	reCommit      *regexp.Regexp

	onCommit     func(transID string, events []*TraceEvent)
	targetDBBase string
	ownAppName   string
	appNames     map[string]string // ConnID -> AppName
}

func NewParser(targetDBBase, ownAppName string, onCommit func(string, []*TraceEvent)) *Parser {
	p := &Parser{
		connections:      make(map[string]string),
		transactions:     make(map[string]string),
		pendingEvents:    make(map[string][]*TraceEvent),
		lastEventPerConn: make(map[string]string),
		lastOPPerConn:    make(map[string]*TraceEvent),

		reHeader:      regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{4}\s+\((\d+):(\w+)\)\s+([A-Z_]+)`),
		reAttach:      regexp.MustCompile(`^\s+([a-zA-Z]:.*|\/.*)\s+\(user.*`),
		reTransaction: regexp.MustCompile(`^\s+Transaction\s+(\d+)`),
		reStatement:   regexp.MustCompile(`(?i)^\s+(INSERT|UPDATE|DELETE)\s+INTO\s+([A-Z0-9_$]+)`),
		reCommit:      regexp.MustCompile(`^\s+Transaction\s+(\d+),\s+duration`),

		ownAppName: ownAppName,
		onCommit:   onCommit,
		appNames:   make(map[string]string),
	}
	p.targetDBBase = p.extractBaseName(targetDBBase)
	return p
}

func (p *Parser) extractBaseName(dsn string) string {
	parts := strings.Split(dsn, "?")
	pathPart := parts[0]
	lastSlash := strings.LastIndex(pathPart, "/")
	if lastSlash == -1 {
		lastSlash = strings.LastIndex(pathPart, "\\")
	}
	if lastSlash != -1 {
		pathPart = pathPart[lastSlash+1:]
	}
	if hostIdx := strings.LastIndex(pathPart, ":"); hostIdx != -1 {
		pathPart = pathPart[hostIdx+1:]
	}
	return strings.ToUpper(pathPart)
}

func (p *Parser) ParseLine(line string) {
	if strings.TrimSpace(line) != "" {
		fmt.Printf("[TRACE RAW] %s\n", line)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	matches := p.reHeader.FindStringSubmatch(line)
	if len(matches) > 3 {
		connID := matches[1]
		event := matches[3]
		p.lastEventPerConn[connID] = event

		if event == "DETACH_DATABASE" {
			delete(p.connections, connID)
		}
		return
	}

	for connID, lastEvent := range p.lastEventPerConn {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		switch lastEvent {
		case "ATTACH_DATABASE":
			if m := p.reAttach.FindStringSubmatch(line); len(m) > 1 {
				path := strings.ToUpper(m[1])
				p.connections[connID] = path
				if appIdx := strings.Index(line, "Application "); appIdx != -1 {
					appPart := line[appIdx+12:]
					if endIdx := strings.Index(appPart, ","); endIdx != -1 {
						p.appNames[connID] = strings.TrimSpace(appPart[:endIdx])
					}
				}
				delete(p.lastEventPerConn, connID)
			}

		case "START_TRANSACTION":
			if m := p.reTransaction.FindStringSubmatch(line); len(m) > 1 {
				transID := m[1]
				p.transactions[transID] = connID
				delete(p.lastEventPerConn, connID)
			}

		case "EXECUTE_STATEMENT_FINISH":
			if m := p.reStatement.FindStringSubmatch(line); len(m) > 2 {
				p.lastOPPerConn[connID] = &TraceEvent{
					Type:  EventType(strings.ToUpper(m[1])),
					Table: strings.ToUpper(m[2]),
					SQL:   trimmed,
				}
			}
			if m := regexp.MustCompile(`Transaction\s+(\d+)`).FindStringSubmatch(line); len(m) > 1 {
				transID := m[1]
				if op, ok := p.lastOPPerConn[connID]; ok {
					op.TransID = transID
					p.pendingEvents[transID] = append(p.pendingEvents[transID], op)
					delete(p.lastOPPerConn, connID)
				}
				delete(p.lastEventPerConn, connID)
			}

		case "COMMIT_TRANSACTION":
			if m := p.reCommit.FindStringSubmatch(line); len(m) > 1 {
				transID := m[1]
				p.handleCommit(transID)
				delete(p.lastEventPerConn, connID)
			}
		}
	}
}

func (p *Parser) handleCommit(transID string) {
	connID, ok := p.transactions[transID]
	if !ok {
		return
	}
	if p.appNames[connID] == p.ownAppName {
		return
	}
	dbPath := p.connections[connID]
	if !strings.Contains(strings.ToUpper(dbPath), p.targetDBBase) {
		return
	}
	events := p.pendingEvents[transID]
	if len(events) > 0 {
		p.onCommit(transID, events)
	}
	delete(p.pendingEvents, transID)
	delete(p.transactions, transID)
}
