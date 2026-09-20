// Package fixtures loads the mhsgrading playthrough fixtures: the captured
// gameplay-log dumps and the expected colors, reason codes and message
// variables that the grading specification's own validation suites use.
// The grader's replay tests import a dump into a scratch MongoDB database,
// run the real scanner + evaluator over it, and compare the stored grades
// with these expectations, so the Go grader and the spec are held to the
// same evidence.
//
// The mhsgrading checkout is located via $MHSGRADING_DIR, falling back to a
// sibling directory of this module (../mhsgrading). Nothing is copied.
package fixtures

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.yaml.in/yaml/v3"
)

// Dir returns the mhsgrading checkout directory.
func Dir() (string, error) {
	if d := os.Getenv("MHSGRADING_DIR"); d != "" {
		return d, nil
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("fixtures: cannot locate module root")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	d := filepath.Join(filepath.Dir(root), "mhsgrading")
	if st, err := os.Stat(filepath.Join(d, "grading-logic")); err != nil || !st.IsDir() {
		return "", fmt.Errorf("fixtures: mhsgrading checkout not found at %s (set MHSGRADING_DIR)", d)
	}
	return d, nil
}

// ColorManifest mirrors rubric-validation/config/fixtures.yaml.
type ColorManifest struct {
	DefaultFixture string                  `yaml:"default_fixture"`
	Fixtures       map[string]ColorFixture `yaml:"fixtures"`
}

// ColorFixture is one dump with the color every point is expected to get.
type ColorFixture struct {
	Log         string            `yaml:"log"`
	Description string            `yaml:"description"`
	Provenance  string            `yaml:"provenance"`
	Expected    map[string]string `yaml:"expected"` // "U2P3" -> "green" | "yellow"
}

// CodeManifest mirrors reason-code-validation/config/expectations.yaml.
type CodeManifest struct {
	DefaultFixture string                 `yaml:"default_fixture"`
	Fixtures       map[string]CodeFixture `yaml:"fixtures"`
}

// CodeFixture is one dump with the reason codes and variables per point.
type CodeFixture struct {
	Log      string                     `yaml:"log"`
	Expected map[string]CodeExpectation `yaml:"expected"` // "U2P3" -> ...
}

// CodeExpectation lists the codes expected to trigger for a point and, for
// each listed code, the variable values the script must return.
type CodeExpectation struct {
	Codes     []string                  `yaml:"codes"`
	Variables map[string]map[string]any `yaml:"variables"`
}

// LoadColors reads the color fixture manifest.
func LoadColors(dir string) (*ColorManifest, error) {
	var m ColorManifest
	if err := readYAML(filepath.Join(dir, "rubric-validation", "config", "fixtures.yaml"), &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// LoadCodes reads the reason-code expectation manifest.
func LoadCodes(dir string) (*CodeManifest, error) {
	var m CodeManifest
	if err := readYAML(filepath.Join(dir, "reason-code-validation", "config", "expectations.yaml"), &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func readYAML(path string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(b, out); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// FixtureIDs returns the manifest's fixture ids, sorted.
func (m *ColorManifest) FixtureIDs() []string {
	ids := make([]string, 0, len(m.Fixtures))
	for id := range m.Fixtures {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// ImportDump loads a stratalog logdata export (a JSON array of documents in
// MongoDB extended JSON) into coll. Returns the number of documents inserted.
func ImportDump(ctx context.Context, coll *mongo.Collection, path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return 0, fmt.Errorf("%s: %w", path, err)
	}
	const batch = 1000
	docs := make([]any, 0, batch)
	total := 0
	flush := func() error {
		if len(docs) == 0 {
			return nil
		}
		if _, err := coll.InsertMany(ctx, docs); err != nil {
			return err
		}
		total += len(docs)
		docs = docs[:0]
		return nil
	}
	for i, r := range raw {
		var d bson.D
		if err := bson.UnmarshalExtJSON(r, false, &d); err != nil {
			return total, fmt.Errorf("%s: record %d: %w", path, i, err)
		}
		docs = append(docs, d)
		if len(docs) == batch {
			if err := flush(); err != nil {
				return total, err
			}
		}
	}
	return total, flush()
}

// PointOrder is the dashboard order of the 26 progress points ("u1p1" …).
var PointOrder = func() []string {
	counts := []int{4, 7, 5, 6, 4}
	var ids []string
	for u, n := range counts {
		for p := 1; p <= n; p++ {
			ids = append(ids, fmt.Sprintf("u%dp%d", u+1, p))
		}
	}
	return ids
}()

// PointID converts a manifest key such as "U2P3" to the grader's "u2p3".
func PointID(manifestKey string) string { return strings.ToLower(manifestKey) }

// ManifestKey converts "u2p3" to "U2P3".
func ManifestKey(pointID string) string { return strings.ToUpper(pointID) }
