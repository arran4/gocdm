package dialog

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// DialogRC represents the parsed configuration of a dialogrc file.
type DialogRC struct {
	Attributes map[string]Attribute
	Strings    map[string]string
	Numbers    map[string]int
	Booleans   map[string]bool
}

// Attribute represents a color attribute: (foreground, background, highlight)
type Attribute struct {
	Foreground string
	Background string
	Highlight  bool
}

// ParseDialogRC reads and parses a dialogrc file.
func ParseDialogRC(path string) (*DialogRC, error) {
	rc := &DialogRC{
		Attributes: make(map[string]Attribute),
		Strings:    make(map[string]string),
		Numbers:    make(map[string]int),
		Booleans:   make(map[string]bool),
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue // ignore invalid lines quietly, or return error? dialog usually ignores
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		if err := parseValue(rc, key, val); err != nil {
			return nil, fmt.Errorf("error on line %d: %w", lineNum, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return rc, nil
}

func parseValue(rc *DialogRC, key, val string) error {
	if val == "" {
		return nil
	}

	// 1. Check String: "string"
	if strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"") {
		rc.Strings[key] = strings.Trim(val, "\"")
		return nil
	}

	// 2. Check Boolean: ON or OFF
	upperVal := strings.ToUpper(val)
	if upperVal == "ON" || upperVal == "OFF" {
		rc.Booleans[key] = upperVal == "ON"
		return nil
	}

	// 3. Check Attribute: (foreground,background,highlight)
	if strings.HasPrefix(val, "(") && strings.HasSuffix(val, ")") {
		inner := val[1 : len(val)-1]
		parts := strings.Split(inner, ",")
		attr := Attribute{}
		if len(parts) >= 1 {
			attr.Foreground = strings.TrimSpace(parts[0])
		}
		if len(parts) >= 2 {
			attr.Background = strings.TrimSpace(parts[1])
		}
		if len(parts) >= 3 {
			hl := strings.ToUpper(strings.TrimSpace(parts[2]))
			attr.Highlight = hl == "ON" || hl == "TRUE"
		}
		rc.Attributes[key] = attr
		return nil
	}

	// 4. Check Number
	if num, err := strconv.Atoi(val); err == nil {
		rc.Numbers[key] = num
		return nil
	}

	// Unrecognized value format, dialog treats it loosely, we will store it as a raw string fallback
	rc.Strings[key] = val
	return nil
}
