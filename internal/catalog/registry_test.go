package catalog

import "testing"

func TestArchitecturesOnlyListOptionsSupportedByShape(t *testing.T) {
	cases := []struct {
		shape string
		want  []string
	}{
		{ShapeMonolith, []string{ArchitectureLayered, ArchitectureClean, ArchitectureDDD}},
		{ShapeMicroservices, []string{ArchitectureLayered, ArchitectureClean, ArchitectureDDD}},
		{ShapeOneShotJob, []string{ArchitecturePipeline}},
	}
	for _, test := range cases {
		t.Run(test.shape, func(t *testing.T) {
			architectures := ArchitecturesForShape(test.shape)
			if len(architectures) != len(test.want) {
				t.Fatalf("got %d architecture choices, want %d", len(architectures), len(test.want))
			}
			for i, architecture := range architectures {
				if architecture.ID != test.want[i] {
					t.Errorf("choice %d = %q, want %q", i, architecture.ID, test.want[i])
				}
			}
		})
	}
}

func TestOneShotJobHasDedicatedPipelineTemplate(t *testing.T) {
	template, ok := TemplateByID(TemplateGoOneShotPipeline)
	if !ok {
		t.Fatal("one-shot pipeline template is missing")
	}
	if template.AppShapeID != ShapeOneShotJob || template.ArchitectureID != ArchitecturePipeline {
		t.Fatalf("unexpected one-shot template metadata: %+v", template)
	}
}

func TestWorkloadsHaveArchitectureDefaultsAndSupportedFirstWave(t *testing.T) {
	for _, workload := range Workloads() {
		architectures := ArchitecturesForWorkload(workload.ID, "")
		if len(architectures) == 0 {
			t.Errorf("workload %q has no architecture", workload.ID)
		}
	}
	for _, id := range []string{CapabilitySQLite, CapabilityPostgres, CapabilityMySQL, CapabilityMongoDB} {
		item, ok := CapabilityByID(id)
		if !ok || !item.Supported {
			t.Errorf("first-wave database %q is not marked supported", id)
		}
	}
	if item, ok := CapabilityByID(CapabilityRedis); !ok || item.Supported {
		t.Fatal("Redis should remain planned until its generator is verified")
	}
}

func TestApplicationPatternCatalogIncludesStandardBackendAndAIPatterns(t *testing.T) {
	for _, id := range []string{PatternBackend, PatternRAG, PatternAgent, PatternChatbot} {
		if _, ok := PatternByID(id); !ok {
			t.Errorf("application pattern %q is missing", id)
		}
	}
}

func TestOnlyGoStackIsSelectableInThisRollout(t *testing.T) {
	setting, ok := SettingByID(SettingStack)
	if !ok || len(setting.Options) != 5 {
		t.Fatalf("stack catalog = %+v", setting)
	}
	if setting.Options[0].ID != StackGo || !setting.Options[0].Supported {
		t.Fatal("Go must be the supported default stack")
	}
	for _, option := range setting.Options[1:] {
		if option.Supported {
			t.Errorf("planned stack %q was marked supported", option.ID)
		}
	}
}
