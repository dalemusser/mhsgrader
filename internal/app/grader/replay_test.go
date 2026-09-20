package grader

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dalemusser/mhsgrader/internal/app/fixtures"
	"github.com/dalemusser/mhsgrader/internal/app/reasoncodes"
	"github.com/dalemusser/mhsgrader/internal/app/store/progressgrades"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// TestFixtureReplay replays the mhsgrading playthrough fixtures through the
// real scanner + evaluator and compares the stored grades with the colors,
// reason codes and message variables the specification's validation suites
// expect. It needs a MongoDB (MHSGRADER_TEST_MONGO_URI, default
// mongodb://localhost:27017) and the mhsgrading checkout (MHSGRADING_DIR or
// ../mhsgrading); it is skipped when either is missing.
//
//	MHSGRADER_TEST_FIXTURES=09-14-26-3,09-03-26-2   restrict the fixtures
//	MHSGRADER_TEST_UNITS=2,3                        assert only these units
func TestFixtureReplay(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	uri := os.Getenv("MHSGRADER_TEST_MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetServerSelectionTimeout(3*time.Second))
	if err != nil {
		t.Skipf("MongoDB not available at %s: %v", uri, err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		t.Skipf("MongoDB not reachable at %s: %v", uri, err)
	}
	defer client.Disconnect(ctx)

	dir, err := fixtures.Dir()
	if err != nil {
		t.Skip(err)
	}
	colors, err := fixtures.LoadColors(dir)
	if err != nil {
		t.Fatal(err)
	}
	codes, err := fixtures.LoadCodes(dir)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := reasoncodes.Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	ids := colors.FixtureIDs()
	if sel := os.Getenv("MHSGRADER_TEST_FIXTURES"); sel != "" {
		ids = strings.Split(sel, ",")
	}
	units := unitFilter()

	for _, id := range ids {
		id := strings.TrimSpace(id)
		cf, ok := colors.Fixtures[id]
		if !ok {
			t.Fatalf("unknown fixture %q", id)
		}
		t.Run(id, func(t *testing.T) {
			replayFixture(ctx, t, client, dir, id, cf, codes.Fixtures[id], catalog, units)
		})
	}
}

func unitFilter() map[int]bool {
	sel := os.Getenv("MHSGRADER_TEST_UNITS")
	if sel == "" {
		return nil
	}
	m := map[int]bool{}
	for _, s := range strings.Split(sel, ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			m[n] = true
		}
	}
	return m
}

func replayFixture(ctx context.Context, t *testing.T, client *mongo.Client, dir, id string, cf fixtures.ColorFixture, codeFx fixtures.CodeFixture, catalog *reasoncodes.Catalog, units map[int]bool) {
	base := fmt.Sprintf("mhsgtest_%s_%d", strings.NewReplacer("-", "_", ".", "_").Replace(id), os.Getpid())
	logDB := client.Database(base + "_log")
	gradesDB := client.Database(base + "_grades")
	defer func() {
		_ = logDB.Drop(context.Background())
		_ = gradesDB.Drop(context.Background())
	}()

	n, err := fixtures.ImportDump(ctx, logDB.Collection("logdata"), filepath.Join(dir, cf.Log))
	if err != nil {
		t.Fatalf("import %s: %v", cf.Log, err)
	}
	userIDs, err := logDB.Collection("logdata").Distinct(ctx, "user_id", bson.M{"game": "mhs"})
	if err != nil || len(userIDs) != 1 {
		t.Fatalf("fixture must hold exactly one player, got %v (err %v)", userIDs, err)
	}
	userID := userIDs[0].(string)
	t.Logf("fixture %s: %d records, player %s", id, n, userID)

	engine := NewEngine(logDB, gradesDB, zap.NewNop(), "mhs", time.Second, 500, 2*time.Minute)
	start := time.Now()
	if err := engine.RunOnce(ctx); err != nil {
		t.Fatalf("replay failed: %v", err)
	}
	t.Logf("graded in %s", time.Since(start).Round(time.Millisecond))

	pg, err := progressgrades.New(gradesDB).GetForUser(ctx, "mhs", userID)
	if err != nil {
		t.Fatal(err)
	}
	if pg == nil {
		t.Fatal("no grade document written")
	}

	var rows []string
	colorOK, colorN, codeOK, codeN := 0, 0, 0, 0
	for _, pid := range fixtures.PointOrder {
		unit, _ := strconv.Atoi(pid[1:strings.Index(pid, "p")])
		if units != nil && !units[unit] {
			continue
		}
		key := fixtures.ManifestKey(pid)
		wantColor := cf.Expected[key]

		items := pg.Grades[pid]
		gotColor, gotCodes := "none", []string{}
		var latest *progressgrades.Grade
		if len(items) > 0 {
			latest = &items[len(items)-1]
			switch latest.Status {
			case "passed":
				gotColor = "green"
			case "flagged":
				gotColor = "yellow"
			default:
				gotColor = latest.Status
			}
			for _, r := range latest.Reasons {
				gotCodes = append(gotCodes, r.Code)
			}
		}

		// A point the player never finished: the spec's scripts default to
		// "yellow" with no reason code (the dashboard shows the not-reached
		// state), while the grader has no final grade at all. Equivalent.
		notReached := false
		if exp, ok := codeFx.Expected[key]; ok && len(exp.Codes) == 0 && wantColor == "yellow" && (gotColor == "none" || gotColor == "active") {
			notReached = true
		}

		colorN++
		cMark := "ok"
		if notReached {
			cMark = "ok (not reached)"
			colorOK++
		} else if gotColor != wantColor {
			cMark = "MISMATCH"
			t.Errorf("%s: color expected %s, got %s (attempts=%d)", pid, wantColor, gotColor, len(items))
		} else {
			colorOK++
		}

		// Reason codes + variables (only when the code fixture covers this point).
		codeMark := "-"
		if exp, ok := codeFx.Expected[key]; ok {
			codeN++
			want := append([]string(nil), exp.Codes...)
			sort.Strings(want)
			got := append([]string(nil), gotCodes...)
			sort.Strings(got)
			problems := []string{}
			if strings.Join(want, ",") != strings.Join(got, ",") {
				problems = append(problems, fmt.Sprintf("codes expected [%s], got [%s]", strings.Join(want, ","), strings.Join(got, ",")))
			}
			for code, vars := range exp.Variables {
				var gotVars map[string]any
				if latest != nil {
					for _, r := range latest.Reasons {
						if r.Code == code {
							gotVars = r.Variables
						}
					}
				}
				for name, wantV := range vars {
					gotV, present := gotVars[name]
					if !present {
						problems = append(problems, fmt.Sprintf("%s.%s missing (expected %v)", code, name, wantV))
					} else if !valuesEqual(gotV, wantV) {
						problems = append(problems, fmt.Sprintf("%s.%s expected %v, got %v", code, name, wantV, gotV))
					}
				}
			}
			if len(problems) == 0 {
				codeOK++
				codeMark = "ok"
			} else {
				codeMark = "MISMATCH"
				t.Errorf("%s: %s", pid, strings.Join(problems, "; "))
			}
		}

		// Every stored reason must be a code the spec defines for this point,
		// and must carry every {placeholder} its instructor message uses.
		if latest != nil {
			spec := catalog.Points[pid]
			for _, r := range latest.Reasons {
				var tmpl string
				known := false
				for _, c := range spec.Codes {
					if c.Code == r.Code {
						known, tmpl = true, c.Message
					}
				}
				if !known {
					t.Errorf("%s: reason code %s is not defined in the spec for this point", pid, r.Code)
					continue
				}
				for _, ph := range reasoncodes.Placeholders(tmpl) {
					if _, ok := r.Variables[ph]; !ok {
						t.Errorf("%s: %s message needs {%s} but the rule did not return it", pid, r.Code, ph)
					}
				}
			}
		}

		reason := ""
		if latest != nil && latest.ReasonCode != "" {
			reason = latest.ReasonCode
		}
		rows = append(rows, fmt.Sprintf("%-5s color %-6s→%-7s %-8s codes %-8s %-26s attempts=%d", pid, wantColor, gotColor, cMark, codeMark, reason, len(items)))
	}
	t.Logf("fixture %s: colors %d/%d, reason codes+variables %d/%d\n%s", id, colorOK, colorN, codeOK, codeN, strings.Join(rows, "\n"))
}

// valuesEqual compares a stored variable with an expected YAML value:
// numbers numerically (any Go/BSON numeric type), everything else by string.
func valuesEqual(got, want any) bool {
	gf, gok := toFloat(got)
	wf, wok := toFloat(want)
	if gok && wok {
		return math.Abs(gf-wf) < 1e-6
	}
	return fmt.Sprint(got) == fmt.Sprint(want)
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case float32:
		return float64(x), true
	case float64:
		return x, true
	}
	return 0, false
}
