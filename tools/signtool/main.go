package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func main() {
	secret := flag.String("secret", "dev_webhook_secret_key_change_in_prod", "Webhook secret (default: dev secret)")
	filePath := flag.String("file", "", "Path to JSON payload file")
	timestamp := flag.String("ts", "", "Custom timestamp (default: current unix timestamp)")
	all := flag.Bool("all", false, "Generate signatures for all sample files and test cases")
	flag.Parse()

	ts := *timestamp
	if ts == "" {
		ts = strconv.FormatInt(time.Now().Unix(), 10)
	}

	if *all {
		generateAll(*secret, ts)
		return
	}

	// Single file or inline string mode
	var rawBody []byte
	var err error

	if *filePath != "" {
		rawBody, err = os.ReadFile(*filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
			os.Exit(1)
		}
	} else if flag.NArg() > 0 {
		rawBody = []byte(flag.Arg(0))
	} else {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  go run tools/signtool/main.go -all")
		fmt.Fprintln(os.Stderr, "  go run tools/signtool/main.go -file samples/payment_success_card.json")
		fmt.Fprintln(os.Stderr, "  go run tools/signtool/main.go '{\"type\":\"PAYMENT_SUCCESS_WEBHOOK\",...}'")
		os.Exit(1)
	}

	sig := computeSignature(*secret, ts, rawBody)
	printResult("Custom Payload", *secret, ts, sig, rawBody, *filePath)
}

func computeSignature(secret, ts string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func printResult(title, secret, ts, sig string, body []byte, filePath string) {
	fmt.Println("════════════════════════════════════════════════════════════════════════════════")
	fmt.Printf("  TEST CASE: %s\n", title)
	fmt.Println("════════════════════════════════════════════════════════════════════════════════")
	fmt.Printf("  x-webhook-timestamp: %s\n", ts)
	fmt.Printf("  x-webhook-signature: %s\n", sig)
	fmt.Println("────────────────────────────────────────────────────────────────────────────────")
	fmt.Println("  JSON Payload:")
	fmt.Println(string(body))
	fmt.Println("════════════════════════════════════════════════════════════════════════════════")
	fmt.Println()
}

func generateAll(secret, ts string) {
	samples := []struct {
		title string
		file  string
	}{
		{"Feature 2: Credit Card Payment Success", "samples/payment_success_card.json"},
		{"Feature 2: UPI Payment Failed", "samples/payment_failed_upi.json"},
		{"Feature 2: Net Banking User Dropped", "samples/payment_dropped_netbanking.json"},
	}

	fmt.Println("╔═══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║     CASHFREE WEBHOOK SIGNATURES FOR ALL FEATURE SAMPLES          ║")
	fmt.Printf("║     Timestamp: %-50s ║\n", ts)
	fmt.Printf("║     Secret:    %-50s ║\n", secret)
	fmt.Println("╚═══════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	for _, s := range samples {
		data, err := os.ReadFile(s.file)
		if err != nil {
			data, err = os.ReadFile(filepath.Join("..", "..", s.file))
			if err != nil {
				fmt.Printf("[SKIP] Could not read %s: %v\n", s.file, err)
				continue
			}
		}

		var compactBuf bytes.Buffer
		_ = json.Compact(&compactBuf, data)

		sig := computeSignature(secret, ts, data)
		printResult(s.title, secret, ts, sig, data, s.file)
	}
}
