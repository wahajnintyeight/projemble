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
