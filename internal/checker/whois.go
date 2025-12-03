package checker

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

// Result represents the result of a domain availability check
type Result struct {
	Domain     string
	Available  bool
	ExpiryDate string
	Error      error
}

// CheckDomain checks if a domain is available using whois
func CheckDomain(domain string) Result {
	result := Result{Domain: domain}

	cmd := exec.Command("whois", domain)
	output, err := cmd.Output()
	if err != nil {
		// whois might return non-zero for some domains, check output anyway
		if output == nil {
			result.Error = err
			return result
		}
	}

	whoisOutput := string(output)

	// First check for clear "not found" / "available" indicators
	availablePatterns := regexp.MustCompile(`(?i)(No match|NOT FOUND|No entries found|No Data Found|not registered|Status:\s*free|Status:\s*available|No Object Found|Domain not found|is free|No information available|not been registered|not exist)`)
	if availablePatterns.MatchString(whoisOutput) {
		result.Available = true
		return result
	}

	// Check for indicators that domain is registered
	registeredPattern := regexp.MustCompile(`(?i)(Name Server|nserver|nameservers|status:\s*active|Registrant|Creation Date|Created:|Domain Name:|Registry Domain ID)`)
	if registeredPattern.MatchString(whoisOutput) {
		result.Available = false
		result.ExpiryDate = extractExpiryDate(whoisOutput)
	} else {
		// If no clear indicators either way, assume available
		result.Available = true
	}

	return result
}

// extractExpiryDate extracts the expiry date from whois output
func extractExpiryDate(whoisOutput string) string {
	// Match various expiry date formats
	expiryPattern := regexp.MustCompile(`(?i)(Expiry Date|Expiration Date|Registry Expiry Date|Expiration Time)[:\s]+([0-9]{4}-[0-9]{2}-[0-9]{2})`)
	matches := expiryPattern.FindStringSubmatch(whoisOutput)
	if len(matches) >= 3 {
		return matches[2]
	}

	// Try to find just the date pattern near expiry keywords
	lines := strings.Split(whoisOutput, "\n")
	datePattern := regexp.MustCompile(`[0-9]{4}-[0-9]{2}-[0-9]{2}`)
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "expir") || strings.Contains(lower, "expiry") {
			if date := datePattern.FindString(line); date != "" {
				return date
			}
		}
	}

	return ""
}

// CheckDomains checks multiple domains concurrently with a worker pool
func CheckDomains(ctx context.Context, domains []string, concurrency int, resultChan chan<- Result) {
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, concurrency)

	for _, domain := range domains {
		select {
		case <-ctx.Done():
			return
		default:
		}

		wg.Add(1)
		semaphore <- struct{}{}

		go func(d string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			select {
			case <-ctx.Done():
				return
			default:
				result := CheckDomain(d)
				select {
				case resultChan <- result:
				case <-ctx.Done():
				}
			}
		}(domain)
	}

	wg.Wait()
}

// CheckDomainsWithCallback checks domains and calls a callback for each result
func CheckDomainsWithCallback(ctx context.Context, domains []string, concurrency int, callback func(Result)) {
	resultChan := make(chan Result, len(domains))

	go func() {
		CheckDomains(ctx, domains, concurrency, resultChan)
		close(resultChan)
	}()

	for result := range resultChan {
		select {
		case <-ctx.Done():
			return
		default:
			callback(result)
		}
	}
}
