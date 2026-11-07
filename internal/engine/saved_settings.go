package engine

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var engineSettingsStart = regexp.MustCompile(`(?m)^LamdaEngineDB\s*=\s*\{`)

// WoW serializes settings as ["key"] = value. Sim/UI modules share this table
// and may save nested diagnostics, selections, or resume state. Only direct
// fields count as engine settings; nested keys and string contents never do.
// This scans data without loading Lua, and retains existing per-setting checks.
func savedEngineFields(data []byte) (map[string]string, error) {
	start := engineSettingsStart.FindIndex(data)
	if start == nil {
		return nil, fmt.Errorf("missing engine settings")
	}
	s := savedSettingsScanner{text: string(data), pos: start[1]}
	values := map[string]string{}
	for {
		if err := s.space(); err != nil {
			return nil, err
		}
		if s.take('}') {
			return values, nil
		}
		if !s.take('[') {
			return nil, fmt.Errorf("incomplete engine settings key")
		}
		if err := s.space(); err != nil {
			return nil, err
		}
		keyStart := s.pos
		if err := s.quoted(); err != nil {
			return nil, err
		}
		key, err := strconv.Unquote(s.text[keyStart:s.pos])
		if err != nil || key == "" {
			return nil, fmt.Errorf("invalid engine settings key")
		}
		if err := s.space(); err != nil {
			return nil, err
		}
		if !s.take(']') {
			return nil, fmt.Errorf("incomplete engine settings key")
		}
		if err := s.space(); err != nil {
			return nil, err
		}
		if !s.take('=') {
			return nil, fmt.Errorf("missing engine settings value")
		}
		if err := s.space(); err != nil {
			return nil, err
		}
		valueStart := s.pos
		if s.pos == len(s.text) {
			return nil, fmt.Errorf("incomplete engine settings")
		}
		switch s.text[s.pos] {
		case '{':
			if err := s.table(); err != nil {
				return nil, err
			}
		case '"', '\'':
			if err := s.quoted(); err != nil {
				return nil, err
			}
		default:
			for s.pos < len(s.text) && !strings.ContainsRune(",;}\r\n", rune(s.text[s.pos])) {
				if strings.HasPrefix(s.text[s.pos:], "--") {
					break
				}
				s.pos++
			}
			literal := strings.TrimSpace(s.text[valueStart:s.pos])
			if literal != "true" && literal != "false" && literal != "nil" {
				if _, err := strconv.ParseFloat(literal, 64); err != nil {
					return nil, fmt.Errorf("invalid scalar setting %s", key)
				}
			}
		}
		value := strings.TrimSpace(s.text[valueStart:s.pos])
		if _, exists := values[key]; exists || value == "" {
			return nil, fmt.Errorf("duplicate or empty setting %s", key)
		}
		// Keep a non-scalar sentinel: a table under a required scalar setting
		// must fail its normal type validation, not masquerade as absent.
		if value[0] == '{' {
			value = "{}"
		}
		values[key] = value
		if err := s.space(); err != nil {
			return nil, err
		}
		if s.take(',') || s.take(';') {
			continue
		}
		if s.take('}') {
			return values, nil
		}
		return nil, fmt.Errorf("incomplete engine settings separator")
	}
}

type savedSettingsScanner struct {
	text string
	pos  int
}

func (s *savedSettingsScanner) take(char byte) bool {
	if s.pos < len(s.text) && s.text[s.pos] == char {
		s.pos++
		return true
	}
	return false
}

func (s *savedSettingsScanner) quoted() error {
	if s.pos == len(s.text) || (s.text[s.pos] != '"' && s.text[s.pos] != '\'') {
		return fmt.Errorf("expected quoted settings string")
	}
	quote := s.text[s.pos]
	s.pos++
	for s.pos < len(s.text) {
		char := s.text[s.pos]
		s.pos++
		if char == quote {
			return nil
		}
		if char == '\n' || char == '\r' {
			return fmt.Errorf("unescaped newline in settings string")
		}
		if char == '\\' && s.pos < len(s.text) {
			s.pos++
		}
	}
	return fmt.Errorf("unterminated settings string")
}

// Comments and long strings can contain braces too. Never let them change the
// table depth, including diagnostics containing copied item links or Lua text.
func (s *savedSettingsScanner) longString() (bool, error) {
	if s.pos >= len(s.text) || s.text[s.pos] != '[' {
		return false, nil
	}
	end := s.pos + 1
	for end < len(s.text) && s.text[end] == '=' {
		end++
	}
	if end == len(s.text) || s.text[end] != '[' {
		return false, nil
	}
	close := "]" + s.text[s.pos+1:end] + "]"
	offset := strings.Index(s.text[end+1:], close)
	if offset < 0 {
		return true, fmt.Errorf("unterminated settings long string")
	}
	s.pos = end + 1 + offset + len(close)
	return true, nil
}

func (s *savedSettingsScanner) space() error {
	for s.pos < len(s.text) {
		if strings.ContainsRune(" \t\r\n", rune(s.text[s.pos])) {
			s.pos++
			continue
		}
		if !strings.HasPrefix(s.text[s.pos:], "--") {
			break
		}
		s.pos += 2
		if found, err := s.longString(); found {
			if err != nil {
				return err
			}
			continue
		}
		for s.pos < len(s.text) && s.text[s.pos] != '\n' {
			s.pos++
		}
	}
	return nil
}

func (s *savedSettingsScanner) table() error {
	depth := 0
	for s.pos < len(s.text) {
		if err := s.space(); err != nil {
			return err
		}
		if s.pos == len(s.text) {
			break
		}
		switch s.text[s.pos] {
		case '"', '\'':
			if err := s.quoted(); err != nil {
				return err
			}
		case '[':
			found, err := s.longString()
			if err != nil {
				return err
			}
			if !found {
				s.pos++
			}
		case '{':
			depth++
			s.pos++
			if depth > 64 {
				return fmt.Errorf("settings nesting exceeds limit")
			}
		case '}':
			depth--
			s.pos++
			if depth == 0 {
				return nil
			}
		default:
			s.pos++
		}
	}
	return fmt.Errorf("unterminated nested settings")
}
