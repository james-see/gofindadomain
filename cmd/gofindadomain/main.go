package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	gofindadomain "github.com/james-see/gofindadomain"
	"github.com/james-see/gofindadomain/internal/checker"
	"github.com/james-see/gofindadomain/internal/tld"
	"github.com/james-see/gofindadomain/internal/tui"
	"github.com/spf13/cobra"
)

const banner = `
  ___      ___ _         _   _   ___                  _      
 / __|___ | __(_)_ _  __| | /_\ |   \ ___ _ __  __ _(_)_ _  
| (_ / _ \| _|| | ' \/ _` + "`" + `| / _ \| |) / _ \ '  \/ _` + "`" + `| | ' \ 
 \___\___/|_| |_|_||_\__,_/_/ \_\___/\___/_|_|_\__,_|_|_||_|
                   Domain Availability Checker
`

// ANSI colors
const (
	reset  = "\033[0m"
	red    = "\033[0;31m"
	green  = "\033[0;32m"
	orange = "\033[0;33m"
	bold   = "\033[1m"
	bGreen = "\033[1;32m"
	bRed   = "\033[1;31m"
)

var (
	keyword     string
	singleTLD   string
	tldFile     string
	onlyAvail   bool
	updateTLD   bool
	interactive bool
	concurrency int
)

var rootCmd = &cobra.Command{
	Use:   "gofindadomain",
	Short: "Domain availability checker",
	Long:  banner + "\nCheck domain availability across multiple TLDs using whois lookups.",
	RunE:  run,
}

func init() {
	rootCmd.Flags().StringVarP(&keyword, "keyword", "k", "", "Keyword to check (e.g., mycompany)")
	rootCmd.Flags().StringVarP(&singleTLD, "tld", "e", "", "Single TLD to check (e.g., .com)")
	rootCmd.Flags().StringVarP(&tldFile, "tld-file", "E", "", "File containing TLDs to check")
	rootCmd.Flags().BoolVarP(&onlyAvail, "not-registered", "x", false, "Only show available domains")
	rootCmd.Flags().BoolVar(&updateTLD, "update-tld", false, "Update TLD list from IANA")
	rootCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Launch interactive TUI mode")
	rootCmd.Flags().IntVarP(&concurrency, "concurrency", "c", 30, "Number of concurrent checks")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Check for whois
	if _, err := exec.LookPath("whois"); err != nil {
		return fmt.Errorf("whois not installed. You must install whois to use this tool")
	}

	// Handle --update-tld
	if updateTLD {
		if keyword != "" || singleTLD != "" || tldFile != "" || onlyAvail || interactive {
			return fmt.Errorf("--update-tld cannot be used with other flags")
		}
		fmt.Println("Fetching TLD data from IANA...")
		if err := tld.UpdateTLDFile("tlds.txt"); err != nil {
			return err
		}
		fmt.Println("TLDs have been saved to tlds.txt")
		return nil
	}

	// Interactive mode
	if interactive {
		tlds := loadTLDs()
		return tui.Run(tlds)
	}

	// CLI mode - validate args
	if keyword == "" {
		return fmt.Errorf("keyword is required (-k). Use -h for help")
	}

	if singleTLD != "" && tldFile != "" {
		return fmt.Errorf("you can only specify one of -e or -E options")
	}

	if singleTLD == "" && tldFile == "" {
		return fmt.Errorf("either -e or -E option is required")
	}

	// Print banner
	fmt.Print(banner)
	fmt.Println()

	// Load TLDs
	var tlds []string
	if singleTLD != "" {
		// Ensure TLD starts with dot
		if !strings.HasPrefix(singleTLD, ".") {
			singleTLD = "." + singleTLD
		}
		tlds = []string{singleTLD}
	} else {
		var err error
		tlds, err = tld.LoadTLDsFromFile(tldFile)
		if err != nil {
			return fmt.Errorf("TLD file %s not found: %w", tldFile, err)
		}
	}

	// Build domain list
	var domains []string
	for _, t := range tlds {
		domains = append(domains, keyword+t)
	}

	// Check domains
	ctx := context.Background()
	checker.CheckDomainsWithCallback(ctx, domains, concurrency, func(result checker.Result) {
		printResult(result, onlyAvail)
	})

	return nil
}

func loadTLDs() []string {
	// Try to load from file first
	if tlds, err := tld.LoadTLDsFromFile("tlds.txt"); err == nil && len(tlds) > 0 {
		return tlds
	}
	// Fall back to embedded
	return tld.LoadTLDsFromString(gofindadomain.EmbeddedTLDs)
}

func printResult(r checker.Result, showOnlyAvail bool) {
	if r.Error != nil {
		fmt.Printf("[%serror%s] %s - %v\n", red, reset, r.Domain, r.Error)
		return
	}

	if r.Available {
		fmt.Printf("[%savail%s] %s\n", bGreen, reset, r.Domain)
		return
	}

	if showOnlyAvail {
		return
	}

	if r.ExpiryDate != "" {
		fmt.Printf("[%staken%s] %s - Exp Date: %s%s%s\n", bRed, reset, r.Domain, orange, r.ExpiryDate, reset)
	} else {
		fmt.Printf("[%staken%s] %s - No expiry date found\n", bRed, reset, r.Domain)
	}
}
