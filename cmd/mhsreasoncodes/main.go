// Command mhsreasoncodes writes the reason-code catalog (instructor-message
// templates and teacher guidance per progress point) as JSON, extracted from
// the grading specification's markdown. The output is embedded by StrataHub
// (internal/app/resources/mhs_reason_codes.json) to render the dashboard's
// review pop-up.
//
//	go run ./cmd/mhsreasoncodes -mhsgrading ../mhsgrading -o ../stratahub/internal/app/resources/mhs_reason_codes.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/dalemusser/mhsgrader/internal/app/fixtures"
	"github.com/dalemusser/mhsgrader/internal/app/reasoncodes"
)

func main() {
	dir := flag.String("mhsgrading", "", "path to the mhsgrading checkout (default: $MHSGRADING_DIR or ../mhsgrading)")
	out := flag.String("o", "", "output file (default: stdout)")
	flag.Parse()

	if *dir == "" {
		d, err := fixtures.Dir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		*dir = d
	}
	cat, err := reasoncodes.Load(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	b, err := json.MarshalIndent(cat, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	b = append(b, '\n')
	if *out == "" {
		os.Stdout.Write(b)
		return
	}
	if err := os.WriteFile(*out, b, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d points)\n", *out, len(cat.Points))
}
