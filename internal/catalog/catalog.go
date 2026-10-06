package catalog

const (
	StackGo = "go"

	ShapeMonolith      = "monolith"
	ShapeMicroservices = "microservices"
	ShapeOneShotJob    = "one-shot-job"

	ArchitectureLayered  = "layered"
	ArchitectureClean    = "clean-hexagonal"
	ArchitectureDDD      = "ddd"
	ArchitecturePipeline = "pipeline"

	TemplateGoMonolithLayered      = "go-monolith-layered"
	TemplateGoMonolithClean        = "go-monolith-clean-hexagonal"
	TemplateGoMonolithDDD          = "go-monolith-ddd"
	TemplateGoMicroservicesLayered = "go-microservices-layered"
	TemplateGoMicroservicesClean   = "go-microservices-clean-hexagonal"
	TemplateGoMicroservicesDDD     = "go-microservices-ddd"
	TemplateGoOneShotPipeline      = "go-one-shot-pipeline"

	SettingStack            = "stack"
	SettingServiceFramework = "service-framework"
)

type AppShape struct {
	ID          string
	Name        string
	Description string
}

type Architecture struct {
	ID          string
	Name        string
	Description string
}

type Template struct {
	ID                 string
	Name               string
	Description        string
	StackID            string
	AppShapeID         string
	ArchitectureID     string
	ServiceFrameworkID string
}

type SettingOption struct {
	ID          string
	Name        string
	Description string
}

type Setting struct {
	ID      string
	Name    string
	Options []SettingOption
}
