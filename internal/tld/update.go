package tld

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const IANAURL = "https://data.iana.org/TLD/tlds-alpha-by-domain.txt"

// UpdateTLDFile fetches the latest TLD list from IANA and saves it to the specified file
func UpdateTLDFile(filepath string) error {
	resp, err := http.Get(IANAURL)
	if err != nil {
		return fmt.Errorf("failed to fetch TLD list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch TLD list: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Process the TLD list
	var tlds []string
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		line := scanner.Text()
		// Skip comments
		if strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Convert to lowercase and add dot prefix
		tld := "." + strings.ToLower(line)
		tlds = append(tlds, tld)
	}

	// Write to file
	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	for _, tld := range tlds {
		fmt.Fprintln(file, tld)
	}

	return nil
}

// LoadTLDsFromFile loads TLDs from a file
func LoadTLDsFromFile(filepath string) ([]string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open TLD file: %w", err)
	}
	defer file.Close()

	var tlds []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			tlds = append(tlds, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read TLD file: %w", err)
	}

	return tlds, nil
}

// LoadTLDsFromString parses TLDs from a string (for embedded data)
func LoadTLDsFromString(data string) []string {
	var tlds []string
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			tlds = append(tlds, line)
		}
	}
	return tlds
}

