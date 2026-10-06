package catalog

const (
	StackGo = "go"

	ShapeMonolith      = "monolith"
	ShapeMicroservices = "microservices"

	ArchitectureLayered = "layered"
	ArchitectureClean   = "clean-hexagonal"
	ArchitectureDDD     = "ddd"

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
