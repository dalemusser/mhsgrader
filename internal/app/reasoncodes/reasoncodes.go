// Package reasoncodes extracts the teacher-facing reason-code templates from
// the grading specification: for every progress point, the `## Reason Codes`
// section of mhsgrading/grading-logic/mhs-unitN-pointM-grading.md — each
// `### CODE` heading with its **Instructor Message** template (whose
// `{placeholders}` the grader fills from Reason.Variables) and the point's
// Teacher Guidance. The dashboard renders these; the grader's tests check
// that every placeholder is a variable the rule emits.
package reasoncodes

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Code is one reason code of a point.
type Code struct {
	Code    string `json:"code"`
	Message string `json:"message"` // template with {placeholders}
}

// Point is the reason-code section of one progress point.
type Point struct {
	Codes    []Code `json:"codes"`
	Guidance string `json:"guidance,omitempty"` // Teacher Guidance text (markdown-ish plain text)
	Note     string `json:"note,omitempty"`     // e.g. "No reason codes ..." for completion-only points
}

// Catalog is every point's reason codes, keyed by point id ("u2p1").
type Catalog struct {
	Source string           `json:"source"` // where it was generated from
	Points map[string]Point `json:"points"`
}

var (
	fileRe        = regexp.MustCompile(`^mhs-unit(\d+)-point(\d+)-grading\.md$`)
	codeHeadingRe = regexp.MustCompile(`^### ([A-Z][A-Z0-9_]+)\s*$`)
	placeholderRe = regexp.MustCompile(`\{([a-z][a-z0-9_]*)\}`)
)

// Load parses every point-grading file under <mhsgradingDir>/grading-logic.
func Load(mhsgradingDir string) (*Catalog, error) {
	dir := filepath.Join(mhsgradingDir, "grading-logic")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	cat := &Catalog{Source: "mhsgrading/grading-logic", Points: map[string]Point{}}
	for _, e := range entries {
		m := fileRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		pt, err := parseFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		cat.Points["u"+m[1]+"p"+m[2]] = pt
	}
	if len(cat.Points) == 0 {
		return nil, fmt.Errorf("no grading files found in %s", dir)
	}
	return cat, nil
}

// parseFile mirrors reason-code-validation/rc_common.parse_reason_codes.
func parseFile(path string) (Point, error) {
	f, err := os.Open(path)
	if err != nil {
		return Point{}, err
	}
	defer f.Close()

	pt := Point{Codes: []Code{}}
	inSection, inCode, inGuidance := false, false, false
	var guidance []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !inSection {
			if strings.TrimSpace(line) == "## Reason Codes" {
				inSection = true
			}
			continue
		}
		if strings.HasPrefix(line, "## ") {
			break // next top-level section
		}
		if strings.HasPrefix(line, "```") {
			inCode = !inCode
			continue
		}
		if inCode {
			continue
		}
		if m := codeHeadingRe.FindStringSubmatch(line); m != nil {
			pt.Codes = append(pt.Codes, Code{Code: m[1]})
			inGuidance = false
			continue
		}
		if strings.HasPrefix(line, "### ") {
			title := strings.ToLower(strings.TrimRight(strings.TrimSpace(line[4:]), ":"))
			inGuidance = strings.HasSuffix(title, "teacher guidance")
			continue
		}
		if strings.HasPrefix(line, "#### ") {
			continue
		}
		if inGuidance {
			guidance = append(guidance, line)
			continue
		}
		if len(pt.Codes) > 0 && strings.Contains(line, "**Instructor Message:**") {
			msg := strings.TrimSpace(strings.SplitN(line, "**Instructor Message:**", 2)[1])
			pt.Codes[len(pt.Codes)-1].Message = msg
			continue
		}
		if pt.Note == "" && strings.HasPrefix(line, "> No reason codes") {
			pt.Note = strings.TrimSpace(line[1:])
		}
	}
	if err := sc.Err(); err != nil {
		return Point{}, err
	}
	if !inSection {
		return Point{}, fmt.Errorf("no '## Reason Codes' section")
	}
	for _, c := range pt.Codes {
		if c.Message == "" {
			return Point{}, fmt.Errorf("reason code %s has no **Instructor Message:** line", c.Code)
		}
	}
	pt.Guidance = strings.TrimSpace(strings.Join(guidance, "\n"))
	return pt, nil
}

// Placeholders returns the ordered, de-duplicated {variable} names of a template.
func Placeholders(message string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range placeholderRe.FindAllStringSubmatch(message, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}

// Render fills the template's placeholders from variables; unknown
// placeholders are left in place.
func Render(message string, variables map[string]any) string {
	return placeholderRe.ReplaceAllStringFunc(message, func(ph string) string {
		name := ph[1 : len(ph)-1]
		if v, ok := variables[name]; ok {
			if v == nil {
				return ""
			}
			return fmt.Sprint(v)
		}
		return ph
	})
}

// PointIDs returns the catalog's point ids in dashboard order.
func (c *Catalog) PointIDs() []string {
	ids := make([]string, 0, len(c.Points))
	for id := range c.Points {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		var ui, pi, uj, pj int
		fmt.Sscanf(ids[i], "u%dp%d", &ui, &pi)
		fmt.Sscanf(ids[j], "u%dp%d", &uj, &pj)
		if ui != uj {
			return ui < uj
		}
		return pi < pj
	})
	return ids
}
