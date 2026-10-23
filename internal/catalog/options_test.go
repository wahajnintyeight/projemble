package catalog

import (
	"strings"
	"testing"
	"time"
)

func httpSelection(topology string, choices map[string]string) Selection {
	return SelectionFor(WorkloadHTTPAPI, topology, choices)
}

func TestResolveAppliesTodaysDefaults(t *testing.T) {
	plan, err := Resolve(httpSelection(TopologyMonolith, nil), "")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Choices[StepDatabase] != OptionNone || len(plan.Modules) != 0 {
		t.Fatalf("monolith default plan = %+v", plan)
	}
	if _, ok := plan.Choices[StepServiceFramework]; ok {
		t.Fatal("service framework must not apply to a monolith")
	}
	if plan.GoVersion != GoBaseline {
		t.Fatalf("go version = %s", plan.GoVersion)
	}

	plan, err = Resolve(httpSelection(TopologyMicroservices, nil), PresetRecommended)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Choices[StepServiceFramework] != "go-micro" || len(plan.Modules) != 1 || plan.Modules[0].Path != "go-micro.dev/v6" {
		t.Fatalf("microservices plan = %+v", plan)
	}
}

func TestResolveCollectsModulesFromChoices(t *testing.T) {
	plan, err := Resolve(httpSelection(TopologyMicroservices, map[string]string{StepDatabase: CapabilityPostgres}), PresetMinimal)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, module := range plan.Modules {
		paths = append(paths, module.Path)
	}
	if strings.Join(paths, ",") != "go-micro.dev/v6,github.com/jackc/pgx/v5" {
		t.Fatalf("modules = %v", paths)
	}
}

func TestResolveReportsEveryProblemAtOnce(t *testing.T) {
	selection := httpSelection(TopologyMonolith, map[string]string{
		StepDatabase:         CapabilityScyllaDB,
		StepServiceFramework: "go-micro",
		"made-up-step":       "x",
	})
	_, err := Resolve(selection, "no-such-preset")
	if err == nil {
		t.Fatal("expected problems")
	}
	for _, want := range []string{`unknown preset "no-such-preset"`, `unknown step "made-up-step"`, "Service framework does not apply", "ScyllaDB is planned"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q:\n%v", want, err)
		}
	}
}

func TestResolveRejectsUnknownOptionAndPlannedOption(t *testing.T) {
	_, err := Resolve(httpSelection(TopologyMonolith, map[string]string{StepDatabase: "oracle"}), "")
	if err == nil || !strings.Contains(err.Error(), `unknown option "oracle"`) {
		t.Fatalf("err = %v", err)
	}
}

func TestResolveHonoursRequiresConflictsAndReplaces(t *testing.T) {
	original := steps
	t.Cleanup(func() { steps = original })
	steps = []Step{
		{ID: "a", Name: "A", Options: []Option{
			{ID: "one", Name: "One", Status: StatusRecommended, Requires: []string{"b=two"}, Conflicts: []string{"c=three"}, Replaces: []string{"d"}},
		}},
		{ID: "b", Name: "B", Options: []Option{{ID: "two", Name: "Two", Status: StatusRecommended}, {ID: "other", Name: "Other", Status: StatusSupported}}},
		{ID: "c", Name: "C", Options: []Option{{ID: "three", Name: "Three", Status: StatusRecommended}}},
		{ID: "d", Name: "D", Options: []Option{{ID: "four", Name: "Four", Status: StatusRecommended}, {ID: OptionNone, Name: "None", Status: StatusSupported}}},
	}

	_, err := Resolve(Selection{}, "")
	if err == nil {
		t.Fatal("expected conflict and replacement problems")
	}
	for _, want := range []string{"One conflicts with c=three"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q:\n%v", want, err)
		}
	}

	_, err = Resolve(Selection{"b": "other", "c": OptionNone, "d": "four"}, "")
	if err == nil {
		t.Fatal("expected requirement problem")
	}
	for _, want := range []string{"One requires b=two", "d is handled by a"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q:\n%v", want, err)
		}
	}
}

func TestEveryOptionModuleIsInTheRegistry(t *testing.T) {
	for _, step := range steps {
		for _, option := range step.Options {
			for _, path := range option.Modules {
				if _, ok := ModuleByPath(path); !ok {
					t.Errorf("%s/%s references unregistered module %s", step.ID, option.ID, path)
				}
			}
		}
	}
}

func TestDatabaseOptionsMirrorSupportedCapabilities(t *testing.T) {
	step, _ := StepByID(StepDatabase)
	for _, capability := range capabilities {
		if capability.Category != "Primary database" {
			continue
		}
		option, ok := step.option(capability.ID)
		if !ok {
			t.Errorf("database step is missing %s", capability.ID)
			continue
		}
		if option.Selectable() != capability.Supported {
			t.Errorf("%s selectable = %v, capability supported = %v", capability.ID, option.Selectable(), capability.Supported)
		}
	}
}

func TestModulePinsAreFresh(t *testing.T) {
	if stale := StaleModules(time.Now(), 120*24*time.Hour); len(stale) > 0 {
		t.Fatalf("module pins older than 120 days, re-verify them: %v", stale)
	}
}

func TestGoDirective(t *testing.T) {
	cases := []struct {
		name string
		in   []ModuleInfo
		want string
	}{
		{"no modules uses the baseline", nil, "1.26.0"},
		{"module without a minimum keeps the baseline", []ModuleInfo{{Path: "a"}}, "1.26.0"},
		{"lower minimum keeps the baseline", []ModuleInfo{{MinGo: "1.24.0"}}, "1.26.0"},
		{"higher minimum raises the line", []ModuleInfo{{MinGo: "1.27.1"}}, "1.27.1"},
		{"highest minimum wins", []ModuleInfo{{MinGo: "1.27.0"}, {MinGo: "1.28.2"}, {MinGo: "1.26.5"}}, "1.28.2"},
		{"comparison is numeric", []ModuleInfo{{MinGo: "1.100.0"}, {MinGo: "1.27.0"}}, "1.100.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := GoDirective(tc.in); got != tc.want {
				t.Fatalf("GoDirective() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCompareGoVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.26.0", "1.26.0", 0},
		{"go1.25.0", "1.26.0", -1},
		{"go1.27.3", "1.26.0", 1},
		{"go1.27rc1", "1.27.0", 0},
		{"1.9.0", "1.10.0", -1},
	}
	for _, tc := range cases {
		if got := CompareGoVersions(tc.a, tc.b); got != tc.want {
			t.Errorf("CompareGoVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestRuntimeLoggerPresets(t *testing.T) {
	for _, tc := range []struct{ preset, explicit, want string }{
		{"", "", "fmt"}, {PresetMinimal, "", "fmt"}, {PresetRecommended, "", "slog"}, {PresetRecommended, "fmt", "fmt"},
	} {
		choices := map[string]string{}
		if tc.explicit != "" {
			choices[StepLogging] = tc.explicit
		}
		plan, err := Resolve(httpSelection(TopologyMonolith, choices), tc.preset)
		if err != nil || plan.Choices[StepLogging] != tc.want {
			t.Fatalf("%+v: %+v %v", tc, plan, err)
		}
	}
}
